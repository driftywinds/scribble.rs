package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/scribble-rs/scribble.rs/internal/api"
	"github.com/scribble-rs/scribble.rs/internal/config"
	"github.com/scribble-rs/scribble.rs/internal/frontend"
	"github.com/scribble-rs/scribble.rs/internal/state"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalln("error loading configuration:", err)
	}

	if cfg.CPUProfilePath != "" {
		log.Println("Starting CPU profiling ....")
		cpuProfileFile, err := os.Create(cfg.CPUProfilePath)
		if err != nil {
			log.Fatal("error creating cpuprofile file:", err)
		}
		if err := pprof.StartCPUProfile(cpuProfileFile); err != nil {
			log.Fatal("error starting cpu profiling:", err)
		}
	}

	// Setting the seed in order for the petnames to be random.
	rand.Seed(time.Now().UnixNano())

	router := chi.NewMux()
	router.Use(middleware.StripSlashes)
	router.Use(middleware.Recoverer)

	api.SetupRoutes(cfg.RootPath, router)

	// FIXME Global state needs to be deleted.
	frontend.SetRootPath(cfg.RootPath)
	frontend.SetupRoutes(cfg.RootPath, router)

	state.LaunchCleanupRoutine()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		defer os.Exit(0)

		log.Printf("Received %s, gracefully shutting down.\n", <-signalChan)

		state.ShutdownLobbiesGracefully()
		if cfg.CPUProfilePath != "" {
			pprof.StopCPUProfile()
			log.Println("Finished CPU profiling.")
		}
	}()

	for _, route := range router.Routes() {
		log.Printf("Registered route: %s\n", route.Pattern)
		if route.SubRoutes != nil {
			for _, subRoute := range route.SubRoutes.Routes() {
				log.Printf("  Registered route: %s\n", subRoute.Pattern)
			}
		}
	}

	address := fmt.Sprintf("%s:%d", cfg.NetworkAddress, cfg.Port)
	log.Println("Started, listening on:", address)

	httpServer := &http.Server{
		Addr:              address,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatalln(httpServer.ListenAndServe())
}
