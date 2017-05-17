package server

import (
	"comrade-pavlik2/pkg/client"
	"comrade-pavlik2/pkg/client/gitlab"
	"comrade-pavlik2/pkg/registry"
	"comrade-pavlik2/pkg/templates"
	"errors"
	"fmt"
	"github.com/go-macaron/bindata"
	"gopkg.in/macaron.v1"
	"net/http"
	"os"
	"strings"
)

// Handler
func GitLabConnector() macaron.Handler {
	return func(w http.ResponseWriter, r *http.Request, ctx *macaron.Context) {
		// create and validate new connection to gitlab
		connection, err := client.NewConnectionFromRequest(r)
		if err != nil && err == gitlab.ErrGitLabInvalidToken {
			writeDenied(ctx)
			return
		}
		if err != nil {
			writeErr(ctx, err)
			return
		}

		// provide mappings for available registries
		ctx.Map(connection)
		ctx.Map(registry.NewComposerRegistry(connection))
		ctx.Map(registry.NewNpmRegistry(connection))

		ctx.Next()
	}
}

// NewServer - return new server instance
func NewServer() *macaron.Macaron {
	// bare server
	m := macaron.New()
	m.Use(macaron.Renderer(macaron.RenderOptions{
		TemplateFileSystem: bindata.Templates(
			bindata.Options{
				Asset:      templates.Asset,
				AssetNames: templates.AssetNames,
				AssetDir:   templates.AssetDir,
				AssetInfo:  templates.AssetInfo,
				Prefix:     "bindata",
			},
		),
	}))
	m.Use(GitLabConnector())
	m.SetAutoHead(true)

	// display cache route
	m.Get("/", func(ctx *macaron.Context, c *client.GitLabConnection) {
		archives, expire := c.GetCachedList()

		ctx.Data["Count"] = len(archives)
		ctx.Data["KGBArchives"] = archives
		ctx.Data["Expire"] = expire.Format("15:04:05")
		ctx.HTML(200, "cached_list")
	})

	// clear cache route
	m.Post("/", func(ctx *macaron.Context, c *client.GitLabConnection) {
		if err := ctx.Req.ParseForm(); err != nil {
			writeErr(ctx, err)
			return
		}

		switch ctx.Req.PostForm.Get("action") {
		case "clear_cache":
			c.ClearCachedList()

		case "update_cache":
			c.EnqueueProjectCache()
		}

		ctx.Redirect("/", 302)
	})

	// disable favicon route
	m.Get("/favicon.ico", func(ctx *macaron.Context) {
		ctx.Resp.WriteHeader(http.StatusNoContent)
		ctx.Resp.Write([]byte(""))
	})

	//
	// COMPOSER PACKAGE MANAGER (WARNING: route order matters)
	// =======================================================
	//
	// real route, display all packages available
	// for provided token.
	//
	m.Get("/packages.json", func(ctx *macaron.Context, r *registry.ComposerRegistry) {
		// @see getPackageDownloadURL function
		endpoint := getPackageDownloadURL(ctx, "/composer/%s/%s.zip")
		pkg, err := r.GetPackageInfoList(endpoint)
		if err != nil {
			writeErr(ctx, err)
			return
		}

		ctx.JSON(200, pkg)
	})

	//
	// real route, serve zip archive
	// for provided token.
	//
	m.Get("/composer/:uuid/:ref.zip", func(ctx *macaron.Context, r *registry.ComposerRegistry) {
		response, err := r.GetPackageArchive(ctx.Params(":uuid"), ctx.Params(":ref"))
		if err != nil {
			writeErr(ctx, err)
			return
		}

		writeOk(ctx, "application/zip", response)
	})

	//
	// NODE.JS PACKAGE MANAGER (WARNING: Route order matters)
	// ======================================================
	//
	// npm search action.
	// with proper .npmrc setup, this route should be never called
	//
	m.Get("/-/*", func(ctx *macaron.Context) {
		writeErr(ctx, errors.New("Invalid .npmrc setup, @see https://github.com/Dalee/comrade-pavlik2"))
	})

	//
	// real route, download package archive
	//
	m.Get("/npm/:uuid/:ref.tgz", func(ctx *macaron.Context, r *registry.NpmRegistry) {
		response, err := r.GetPackageArchive(ctx.Params(":uuid"), ctx.Params(":ref"))
		if err != nil {
			writeErr(ctx, err)
			return
		}

		writeOk(ctx, "application/gzip", response)
	})

	//
	// real route, request package info
	//
	m.Get("/*", func(ctx *macaron.Context, r *registry.NpmRegistry) {
		// @see getPackageDownloadURL function
		endpoint := getPackageDownloadURL(ctx, "/npm/%s/%s.tgz")
		pkg, err := r.GetPackageInfo(ctx.Params("*"), endpoint)
		if err != nil {
			writeErr(ctx, err)
			return
		}

		ctx.JSON(200, pkg)
	})

	return m
}

//
// Private API
//

// Respond with 200 OK, correct mime and json/binary data
func writeOk(ctx *macaron.Context, mime string, data []byte) {
	ctx.Resp.Header().Set("Content-Type", mime)
	ctx.Resp.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	ctx.Resp.WriteHeader(http.StatusOK)
	ctx.Resp.Write(data)
}

// Respond with 500 Internal Server Error when any error is detected
func writeErr(ctx *macaron.Context, err error) {
	data := []byte(err.Error())
	ctx.Resp.Header().Set("Content-Type", "text/plain")
	ctx.Resp.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	ctx.Resp.WriteHeader(http.StatusInternalServerError)
	ctx.Resp.Write(data)
}

// Respond with 401 Unauthorized each time when request doesn't contain any kind of authorization
func writeDenied(ctx *macaron.Context) {
	data := []byte("Unauthorized")
	ctx.Resp.Header().Set("WWW-Authenticate", "Basic realm=\"Comrade Pavlik\"")
	ctx.Resp.Header().Set("Content-Type", "text/plain")
	ctx.Resp.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	ctx.Resp.WriteHeader(http.StatusUnauthorized)
	ctx.Resp.Write(data)
}

// Try to correctly format public host and download uri for package:
//  * either such host is provided via environment variables
//  * or try to extract scheme://host:port from request.
func getPackageDownloadURL(ctx *macaron.Context, distFmt string) string {
	packageURI := strings.TrimLeft(distFmt, "/")
	publicHost := strings.TrimRight(os.Getenv("PAVLIK_PUBLIC_HOST"), "/")

	// auto-guess publicHost: scheme://hostname
	if publicHost == "" {
		scheme := ctx.Req.URL.Scheme
		if scheme == "" {
			scheme = "http"
		}

		host := ctx.Req.Host
		if host == "" {
			host = "localhost"
		}

		publicHost = fmt.Sprintf("%s://%s", scheme, strings.TrimRight(host, "/"))
	}

	return fmt.Sprintf("%s/%s", publicHost, packageURI)
}
