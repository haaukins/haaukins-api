package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/aau-network-security/haaukins-api/app"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	defaultConfigFile = "config.yml"
)

func handleCancel(clean func() error) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Info().Msgf("Shutting down gracefully...")
		if err := clean(); err != nil {
			log.Error().Msgf("Error while shutting down: %s", err)
			os.Exit(1)
		}
		log.Info().Msgf("Closed API")
		os.Exit(0)
	}()
}

func main() {
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	confFilePtr := flag.String("config", defaultConfigFile, "configuration file")
	flag.Parse()

	c, err := app.NewConfigFromFile(*confFilePtr)
	if err != nil {
		log.Error().Msgf("unable to read configuration file \"%s\": %s\n", *confFilePtr, err)
		return
	}
	handleCancel(func() error {
		return nil //todo configure the shut down gracefully
	})

	api, err := app.New(c)
	if err != nil {
		log.Error().Msgf("unable to create API: %s\n", err)
		return
	}

	api.Run()

}