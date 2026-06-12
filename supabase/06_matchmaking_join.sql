-- ============================================================================
-- KHEL ARENA — 06: Matchmaking joins (request → author approves)
-- Run after schema.sql + functions.sql + 02 + 03 + 04 + 05.
--
-- Problem: respondToPost() inserted a matchmaking_responses row but never
-- touched filled_players, so the board's "spots left" gauge never moved.
--
-- Model: joining a call files a REQUEST (matchmaking_responses.accepted = false)
-- — it does not take a spot. The post author then approves a request (which
-- fills the spot, using the dormant `accepted` flag the schema already had) or
-- declines it. All spot accounting is atomic under a FOR UPDATE lock on the
-- post, with the not_overfilled CHECK constraint as the final backstop.
-- ============================================================================

-- ----------------------------------------------------------------------------
-- respond_to_matchmaking_post — a player asks to join. Files a pending request;
-- no spot is taken until the author approves.
-- ----------------------------------------------------------------------------
create or replace function public.respond_to_matchmaking_post(
  p_post_id uuid,
  p_message text default null
) returns public.matchmaking_posts
language plpgsql security definer set search_path = public as $$
declare
  v_user uuid := auth.uid();
  v_post public.matchmaking_posts%rowtype;
begin
  if v_user is null then
    raise exception 'AUTH_REQUIRED' using errcode = '28000';
  end if;

  select * into v_post
  from public.matchmaking_posts
  where id = p_post_id
  for update;

  if not found then
    raise exception 'POST_NOT_FOUND';
  end if;
  if v_post.author_id = v_user then
    raise exception 'OWN_POST: you cannot join your own call';
  end if;
  if v_post.status <> 'open' then
    raise exception 'POST_CLOSED: this game is no longer open';
  end if;
  if v_post.starts_at < now() then
    raise exception 'SLOT_IN_PAST: this game has already started';
  end if;

  -- File the request. The author decides; the (post_id, user_id) PK blocks
  -- a second request from the same player.
  insert into public.matchmaking_responses (post_id, user_id, message, accepted)
  values (p_post_id, v_user, left(p_message, 200), false);

  return v_post;

exception
  when unique_violation then
    raise exception 'ALREADY_JOINED: you already asked to join this game';
end;
$$;

-- ----------------------------------------------------------------------------
-- approve_response — author accepts a pending request, which takes a spot and
-- closes the call on the last one.
-- ----------------------------------------------------------------------------
create or replace function public.approve_response(
  p_post_id uuid,
  p_user_id uuid
) returns public.matchmaking_posts
language plpgsql security definer set search_path = public as $$
declare
  v_user    uuid := auth.uid();
  v_post    public.matchmaking_posts%rowtype;
  v_updated int;
begin
  select * into v_post
  from public.matchmaking_posts
  where id = p_post_id
  for update;

  if not found then
    raise exception 'POST_NOT_FOUND';
  end if;
  if v_post.author_id <> v_user then
    raise exception 'NOT_OWNER: only the post author can approve requests';
  end if;
  if v_post.filled_players >= v_post.needed_players then
    raise exception 'POST_FULL: every spot is already taken';
  end if;

  update public.matchmaking_responses
     set accepted = true
   where post_id = p_post_id and user_id = p_user_id and accepted = false;
  get diagnostics v_updated = row_count;
  if v_updated = 0 then
    raise exception 'REQUEST_NOT_FOUND: no pending request from that player';
  end if;

  update public.matchmaking_posts
     set filled_players = filled_players + 1,
         status = case
                    when filled_players + 1 >= needed_players then 'filled'
                    else status
                  end
   where id = p_post_id
  returning * into v_post;

  return v_post;
end;
$$;

-- ----------------------------------------------------------------------------
-- decline_response — author removes a request. If it had already been accepted,
-- the spot is handed back and a full call reopens.
-- ----------------------------------------------------------------------------
create or replace function public.decline_response(
  p_post_id uuid,
  p_user_id uuid
) returns public.matchmaking_posts
language plpgsql security definer set search_path = public as $$
declare
  v_user     uuid := auth.uid();
  v_post     public.matchmaking_posts%rowtype;
  v_accepted boolean;
begin
  select * into v_post
  from public.matchmaking_posts
  where id = p_post_id
  for update;

  if not found then
    raise exception 'POST_NOT_FOUND';
  end if;
  if v_post.author_id <> v_user then
    raise exception 'NOT_OWNER: only the post author can manage requests';
  end if;

  select accepted into v_accepted
  from public.matchmaking_responses
  where post_id = p_post_id and user_id = p_user_id;
  if not found then
    raise exception 'REQUEST_NOT_FOUND: no request from that player';
  end if;

  delete from public.matchmaking_responses
  where post_id = p_post_id and user_id = p_user_id;

  if v_accepted then
    update public.matchmaking_posts
       set filled_players = greatest(filled_players - 1, 0),
           status = case when status = 'filled' then 'open' else status end
     where id = p_post_id
    returning * into v_post;
  end if;

  return v_post;
end;
$$;

grant execute on function public.respond_to_matchmaking_post to authenticated;
grant execute on function public.approve_response to authenticated;
grant execute on function public.decline_response to authenticated;
