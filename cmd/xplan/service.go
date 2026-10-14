package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"

	"github.com/ffqa/xpier-xplan/internal/app"
	"github.com/ffqa/xpier-xplan/internal/review"
	"github.com/ffqa/xpier-xplan/internal/service"
)

const serviceUsage = "usage: xplan service install|uninstall|restart|status"

// cmdService manages xplan as a background service (launchd / systemd --user).
func cmdService(_ context.Context, application *app.App, args []string) error {
	if len(args) != 1 {
		return errors.New(serviceUsage)
	}
	manager, err := service.ForCurrentOS()
	if err != nil {
		return err
	}
	listen := application.Config().Listen
	switch args[0] {
	case "install":
		return installService(application, manager, listen)
	case "uninstall":
		if err := manager.Uninstall(); err != nil {
			return err
		}
		fmt.Printf("xplan background service removed (%s). Start it by hand with `xplan serve`.\n", manager.Name())
		return nil
	case "restart":
		if err := manager.Restart(); err != nil {
			return err
		}
		if err := service.WaitForServer(listen, 20*time.Second); err != nil {
			return err
		}
		fmt.Println("xplan restarted:", "http://"+listen)
		return nil
	case "status":
		return printServiceStatus(manager, listen)
	default:
		return errors.New(serviceUsage)
	}
}

func installService(application *app.App, manager service.Manager, listen string) error {
	spec, err := service.NewSpec(application.Home, listen, review.AugmentedPath())
	if err != nil {
		return err
	}
	current, err := manager.Status()
	if err != nil {
		return err
	}
	if service.PortInUse(listen) && !current.Running {
		return fmt.Errorf("%s is already in use, probably by an `xplan serve` started in a terminal; stop it (Ctrl+C) and run `xplan service install` again", listen)
	}
	if err := manager.Install(spec); err != nil {
		return err
	}
	if err := service.WaitForServer(listen, 20*time.Second); err != nil {
		return fmt.Errorf("%w; see %s", err, spec.LogFile)
	}
	fmt.Printf("xplan now runs in the background (%s): it starts at login, restarts after a crash and runs scheduled reviews.\n", manager.Name())
	fmt.Println("  UI:     http://" + listen)
	fmt.Println("  Binary:", spec.Binary)
	fmt.Println("  Log:   ", spec.LogFile)
	fmt.Println("After rebuilding xplan, run `xplan service restart` to load the new binary.")
	if runtime.GOOS == "linux" {
		fmt.Println("To keep it running after you log out: loginctl enable-linger $USER")
	}
	return nil
}

func printServiceStatus(manager service.Manager, listen string) error {
	status, err := manager.Status()
	if err != nil {
		return err
	}
	fmt.Printf("Service manager: %s\n", status.Manager)
	fmt.Printf("Installed:       %v (%s)\n", status.Installed, status.UnitFile)
	if status.Running {
		fmt.Printf("Running:         yes, pid %d\n", status.PID)
	} else {
		fmt.Println("Running:         no")
	}
	if err := service.WaitForServer(listen, 2*time.Second); err != nil {
		fmt.Printf("Answering:       no (%s)\n", listen)
	} else {
		fmt.Printf("Answering:       yes, http://%s\n", listen)
	}
	return nil
}
