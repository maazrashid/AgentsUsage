package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/maazrashid/AgentsUsage/internal/server"
)

type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
)

type Status struct {
	State     State
	Address   string
	LastError error
}

type Controller struct {
	configuredAddress string
	source            server.DataSource

	mu        sync.RWMutex
	state     State
	address   string
	lastError error
	cancel    context.CancelFunc
	done      chan struct{}
}

func NewController(address string, source server.DataSource) *Controller {
	return &Controller{configuredAddress: address, source: source, state: StateStopped}
}

func (c *Controller) Start() error {
	c.mu.Lock()
	if c.state == StateRunning || c.state == StateStarting {
		c.mu.Unlock()
		return nil
	}
	if c.state == StateStopping {
		c.mu.Unlock()
		return errors.New("server is still stopping")
	}
	c.state = StateStarting
	c.lastError = nil
	c.mu.Unlock()

	listener, err := net.Listen("tcp", c.configuredAddress)
	if err != nil {
		err = fmt.Errorf("start dashboard server: %w", err)
		c.mu.Lock()
		c.state = StateStopped
		c.lastError = err
		c.mu.Unlock()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	httpServer := server.New(c.configuredAddress, c.source)
	c.mu.Lock()
	c.state = StateRunning
	c.address = listener.Addr().String()
	c.cancel = cancel
	c.done = done
	c.mu.Unlock()

	go func() {
		runErr := httpServer.Serve(ctx, listener)
		c.mu.Lock()
		if runErr != nil {
			c.lastError = runErr
		}
		c.state = StateStopped
		c.address = ""
		c.cancel = nil
		c.done = nil
		close(done)
		c.mu.Unlock()
	}()
	return nil
}

func (c *Controller) Stop(ctx context.Context) error {
	c.mu.Lock()
	if c.state == StateStopped {
		c.mu.Unlock()
		return nil
	}
	if c.state == StateStarting {
		c.mu.Unlock()
		return errors.New("server is still starting")
	}
	done := c.done
	if c.state == StateRunning {
		c.state = StateStopping
		c.cancel()
	}
	c.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("stop dashboard server: %w", ctx.Err())
	}
}

func (c *Controller) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Status{State: c.state, Address: c.address, LastError: c.lastError}
}

func (c *Controller) StopWithTimeout() error {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	return c.Stop(ctx)
}
