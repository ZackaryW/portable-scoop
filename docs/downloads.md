# Download plans

The compiler freezes each network URL, filename, digest and extraction selector
before the backend starts. A checked `#/name.zip` or `#/name.exe` fragment chooses
the local basename and is removed from the HTTP request. Query strings remain
part of the request but cannot choose the filename. Without a fragment the last
URL path component supplies the name. Ambiguous encoded separators, duplicate
case-insensitive names, traversal, device names and alternate streams reject.

Downloads pair with hashes and optional `extract_dir` / `extract_to` entries by
index. A single extraction string applies to the first download. Empty and null
selectors leave the corresponding download at the package root. Checksums are
mandatory even for cache hits. SHA256, SHA512, SHA1 and MD5 are supported according
to the manifest declaration; these provide different levels of integrity.

Direct EXE payloads are retained as inert package files. Installer declarations
remain unsupported. Filename acceptance does not prove a payload is portable.
ZIP extraction remains available; other archive readers are a later checkpoint.

The matching runtime/backend contract is `portable-operations-v1.0`. This requires
a matching distribution; replacing individual files in an older Store is unsupported.
