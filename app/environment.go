package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	hlab "github.com/aau-network-security/haaukins/lab"
	"github.com/aau-network-security/haaukins/store"
	"github.com/aau-network-security/haaukins/svcs/guacamole"
	"github.com/aau-network-security/haaukins/virtual/docker"
	"github.com/rs/zerolog/log"
)

type environment struct {
	timer      *time.Time
	challenges []store.Tag
	lab        hlab.Lab
	guacamole  guacamole.Guacamole
	guacPort   uint
}

func (e environment) Close() error {
	panic("implement me")
}

type Environment interface {
	Assign(Client, string) error
	Close() error //close the dockers and the vms
}

func (lm *LearningMaterialAPI) newEnvironment(challenges []store.Tag) (Environment, error) {

	ctx := context.Background()
	exercises, _ := lm.exStore.GetExercisesByTags(challenges...)

	labConf := hlab.Config{
		Exercises: exercises,
		Frontends: lm.frontend,
	}

	lh := hlab.LabHost{
		Vlib: lm.vlib,
		Conf: labConf,
	}

	guac, err := guacamole.New(ctx, guacamole.Config{})
	if err != nil {
		log.Error().Msgf("Error while creating new guacamole %s", err.Error())
		return environment{}, err
	}

	if err := guac.Start(ctx); err != nil {
		log.Error().Msgf("Error while starting guacamole %s", err.Error())
		return environment{}, err
	}

	lab, err := lh.NewLab(ctx)
	if err != nil {
		log.Error().Msgf("Error while creating new lab %s", err.Error())
		return environment{}, err
	}

	if err := lab.Start(ctx); err != nil {
		log.Error().Msgf("Error while starting lab %s", err.Error())
	}

	env := &environment{
		timer:      nil, //todo implement the timer
		challenges: challenges,
		lab:        lab,
		guacamole:  guac,
		guacPort:   guac.GetPort(),
	}

	return env, nil
}

func (e environment) Assign(client Client, chals string) error {

	clientID := client.ID()
	rdpPorts := e.lab.RdpConnPorts()
	if n := len(rdpPorts); n == 0 {
		log.
			Debug().
			Int("amount", n).
			Msg("Too few RDP connections")

		return errors.New("RdpConfErr")
	}

	u := guacamole.GuacUser{
		Username: clientID,
		Password: clientID,
	}

	if err := e.guacamole.CreateUser(u.Username, u.Password); err != nil {
		log.
			Debug().
			Str("err", err.Error()).
			Msg("Unable to create guacamole user")
		return err
	}

	dockerHost := docker.NewHost()
	hostIp, err := dockerHost.GetDockerHostIP()
	if err != nil {
		return err
	}

	for i, port := range rdpPorts {
		num := i + 1
		name := fmt.Sprintf("%s-client%d", clientID, num)

		log.Debug().Str("client", clientID).Uint("port", port).Msg("Creating RDP Connection for group")
		if err := e.guacamole.CreateRDPConn(guacamole.CreateRDPConnOpts{
			Host:     hostIp,
			Port:     port,
			Name:     name,
			GuacUser: u.Username,
			Username: &u.Username,
			Password: &u.Password,
		}); err != nil {
			return err
		}
	}

	content, err := e.guacamole.RawLogin(clientID, clientID)
	if err != nil {
		log.
			Debug().
			Str("err", err.Error()).
			Msg("Unable to login to guacamole")
		return err
	}
	cookie := url.QueryEscape(string(content))

	cc, err := client.GetChallenge(chals)
	if err != nil {
		return errors.New("challenge not found")
	}
	cc.guacPort = e.guacPort
	cc.guacCookie = cookie
	cc.isReady = true

	return nil
}
