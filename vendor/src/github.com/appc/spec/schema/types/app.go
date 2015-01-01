package types

import (
	"encoding/json"
	"errors"
	"fmt"
)

type App struct {
	Exec          Exec              `json:"exec"`
	EventHandlers []EventHandler    `json:"eventHandlers,omitempty"`
	User          string            `json:"user"`
	Group         string            `json:"group"`
	Environment   map[string]string `json:"environment,omitempty"`
	MountPoints   []MountPoint      `json:"mountPoints,omitempty"`
	Ports         []Port            `json:"ports,omitempty"`
	Isolators     []Isolator        `json:"isolators,omitempty"`
}

// app is a model to facilitate extra validation during the
// unmarshalling of the App
type app App

func (a *App) UnmarshalJSON(data []byte) error {
	ja := app{}
	err := json.Unmarshal(data, &ja)
	if err != nil {
		return err
	}
	na := App(ja)
	if err := na.assertValid(); err != nil {
		return err
	}
	if na.Environment == nil {
		na.Environment = make(map[string]string)
	}
	*a = na
	return nil
}

func (a App) MarshalJSON() ([]byte, error) {
	if err := a.assertValid(); err != nil {
		return nil, err
	}
	return json.Marshal(app(a))
}

func (a *App) assertValid() error {
	if err := a.Exec.assertValid(); err != nil {
		return err
	}
	if a.User == "" {
		return errors.New(`User is required`)
	}
	if a.Group == "" {
		return errors.New(`Group is required`)
	}
	eh := make(map[string]bool)
	for _, e := range a.EventHandlers {
		name := e.Name
		if eh[name] {
			return fmt.Errorf("Only one eventHandler of name %q allowed", name)
		}
		eh[name] = true
	}
	return nil
}
