// Avatar image helper. Optimizes real http(s) URLs (e.g. Supabase Storage)
// through next/image; renders object-URL / data previews — like demo-mode
// local file picks — directly, since the optimizer can't process those.
import Image from "next/image";

export default function AvatarImage({
  src,
  size,
  alt = "",
  className = "",
}: {
  src: string;
  size: number;
  alt?: string;
  className?: string;
}) {
  if (!/^https?:\/\//.test(src)) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} width={size} height={size} className={className} />;
  }
  return <Image src={src} alt={alt} width={size} height={size} className={className} />;
}
