The analogy

Think of an interface as a checklist of abilities, not a type in the traditional sense. If I say "I need someone who can drive," I don't care if that person is a "TaxiDriver," a "Parent," or a "RaceCarDriver" — I only care that they have the "can drive" ability on their checklist. If a "RaceCarDriver" can drive and can also change tires and can also read telemetry, that's fine — they still satisfy "someone who can drive," because that's a subset of what they can do.

Go interfaces work exactly like this. An interface doesn't say "you must be exactly this type" — it says "you must have at least these methods." Anything with at least those methods qualifies, no matter what else it can also do.

Applying it here

AuthAPI is the "RaceCarDriver" — it can do more:
type AuthAPI interface {
    Login(ctx context.Context, email, password string, sc postgres.SessionContext) (service.Session, error)
    Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error)
}

Authenticator is "someone who can drive" — it asks for less:
type Authenticator interface {
    Authenticate(accessToken string) (uuid.UUID, domain.AccountType, error)
}

s.authAPI is a variable whose declared type is AuthAPI. But at runtime, it's actually holding a *service.AuthService, which has Login, Authenticate, and a bunch of other methods too (Register, Refresh, etc.). When you write:
withAuth(s.authAPI)
and withAuth expects an Authenticator, Go checks: "does whatever s.authAPI is guaranteed to have include everything Authenticator asks for?" AuthAPI guarantees Login + Authenticate. Authenticator only asks for Authenticate. Since Authenticate is in that guaranteed list, the check passes — Go lets you pass it in directly. No casting, no wrapper function, no boilerplate. It just compiles.

A tiny standalone example, to see the mechanism itself

type Reader interface {
    Read() string
}

type ReadWriter interface {
    Read() string
    Write(string)
}

func printIt(r Reader) {
    fmt.Println(r.Read())
}

func main() {
    var rw ReadWriter = someImplementation{}
    printIt(rw) // works: ReadWriter has Read(), which is all Reader asks for
}
someImplementation never says "I implement Reader" anywhere in its source. There's no implements keyword in Go at all. The compiler just looks at the method sets at the point of the function call and checks compatibility. This is what "structural typing" means — identity by shape (what methods exist), not by declared lineage (what interface you claim to implement).

Why this matters for your specific case

You get to keep one field on Server (authAPI AuthAPI) instead of two, because that one field's type happens to already promise everything both consumers (login handler wanting Login, withAuth middleware wanting Authenticate) need — and Go verifies that automatically at the two different call sites, each checking against the narrower interface it actually cares about, without you writing any connecting code between them.
