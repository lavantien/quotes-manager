package main

import "fmt"

type command struct {
	Name string
	Desc string
	Run  func(args []string) error
}

func commands() []command {
	var all []command
	all = append(all, goCommands()...)
	all = append(all, qualityCommands()...)
	all = append(all, appCommands()...)
	all = append(all, assetCommands()...)
	return all
}

func dispatch(name string, args []string) error {
	for _, c := range commands() {
		if c.Name == name {
			return c.Run(args)
		}
	}
	return fmt.Errorf("unknown subcommand %q", name)
}
