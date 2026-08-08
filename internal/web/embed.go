package web

import "embed"

// Files holds the Vite production build. Run `yarn build` in web before building Go.
//
//go:embed dist
var Files embed.FS
