// Package winsvc integrates the server with the Windows service manager via
// kardianos/service, so the bare .exe installs itself as a service with no
// external tools (NSSM, Task Scheduler).
package winsvc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/kardianos/service"
)

// Defaults used when neither a flag nor .env sets a value. Kept plain on
// purpose: whatever name the operator wants (including any environment
// marker) is theirs to choose, nothing is appended.
const (
	DefaultName        = "HttpProxy"
	DefaultDisplayName = "HTTP Proxy"
	DefaultDescription = "Reverse proxy forwarding every request to TARGET"

	// NameFlag is the flag the installed service is started with, so the
	// running service knows the name it was registered under.
	NameFlag = "name"

	maxNameLen = 256
)

// Actions accepted by Control.
var Actions = []string{"install", "uninstall", "start", "stop", "restart", "run"}

// Options is how the service is registered.
type Options struct {
	Name        string // key in the service manager; used by sc.exe and start/stop
	DisplayName string // shown in services.msc
	Description string
}

// WithDefaults fills empty fields with the built-in defaults. An empty display
// name follows the name, so `--name Foo` alone doesn't show up as the default
// display name.
func (o Options) WithDefaults() Options {
	o.Name = strings.TrimSpace(o.Name)
	o.DisplayName = strings.TrimSpace(o.DisplayName)
	o.Description = strings.TrimSpace(o.Description)
	if o.Name == "" {
		o.Name = DefaultName
		if o.DisplayName == "" {
			o.DisplayName = DefaultDisplayName
		}
	}
	if o.DisplayName == "" {
		o.DisplayName = o.Name
	}
	if o.Description == "" {
		o.Description = DefaultDescription
	}
	return o
}

// Validate rejects names the Windows service manager refuses.
func (o Options) Validate() error {
	switch {
	case o.Name == "":
		return errors.New("service name is empty")
	case strings.ContainsAny(o.Name, `/\`):
		return fmt.Errorf("service name %q must not contain / or \\", o.Name)
	case len(o.Name) > maxNameLen || len(o.DisplayName) > maxNameLen:
		return fmt.Errorf("service name and display name must be at most %d characters", maxNameLen)
	}
	return nil
}

type program struct {
	run    func(ctx context.Context) error
	cancel context.CancelFunc
	done   chan struct{}
}

func (p *program) Start(_ service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		if err := p.run(ctx); err != nil {
			slog.Error("service run failed", "err", err)
		}
	}()
	return nil
}

func (p *program) Stop(_ service.Service) error {
	p.cancel()
	<-p.done
	return nil
}

// Control handles `httpproxy service <action>`. "run" is what the
// service manager invokes (and works interactively for testing); the others
// manage the installed service identified by opts.Name.
func Control(action string, opts Options, run func(ctx context.Context) error) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	cfg := &service.Config{
		Name:        opts.Name,
		DisplayName: opts.DisplayName,
		Description: opts.Description,
		Arguments:   []string{"service", "run", "--" + NameFlag, opts.Name},
	}
	svc, err := service.New(&program{run: run}, cfg)
	if err != nil {
		return err
	}
	if action == "run" {
		return svc.Run()
	}
	if err := service.Control(svc, action); err != nil {
		return fmt.Errorf("service %s %q: %w", action, opts.Name, err)
	}
	fmt.Printf("service %s %q: ok\n", action, opts.Name)
	return nil
}

// Interactive reports whether the process runs from a console rather than
// under the service manager.
func Interactive() bool { return service.Interactive() }
