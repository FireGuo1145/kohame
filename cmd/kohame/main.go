package main

import (
	"flag"
	"log"
	"net/http"

	"kohame/internal/config"
	"kohame/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yml", "path to the YAML configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	app, err := server.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()
	log.Printf("Kohame is listening on http://localhost%s (repositories: %s; database: %s)", cfg.Server.Addr, cfg.Storage.RepositoryRoot, cfg.Database.Driver)
	log.Fatal(http.ListenAndServe(cfg.Server.Addr, app.Handler()))
}
