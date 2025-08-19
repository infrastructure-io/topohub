package ssh

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"

	"golang.org/x/crypto/ssh"

	"github.com/infrastructure-io/topohub/pkg/log"
)

type Client interface {
	// Ping checks if the SSH connection is alive
	Ping() error

	// Run executes a command on the remote host
	Run(cmd string) (string, error)

	// Close closes the SSH connection
	Close() error
}

// Check implantation
var _ Client = (*clientImpl)(nil)

func NewClient(ip string, port int, cfg *ssh.ClientConfig) (Client, error) {
	// Build connection address
	addr := net.JoinHostPort(ip, strconv.Itoa(int(port)))
	// Establish SSH connection
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("dial ssh connection failed, err: %v", err)
	}
	return &clientImpl{conn: conn}, nil
}

type clientImpl struct {
	conn *ssh.Client
}

func (c *clientImpl) Ping() error {
	_, err := c.Run("echo 1")
	return err
}

func (c *clientImpl) Run(cmd string) (string, error) {
	if c.conn == nil {
		return "", errors.New("conn is nil")
	}
	session, err := c.conn.NewSession()
	if err != nil {
		return "", fmt.Errorf("new ssh session failed: %w", err)
	}
	defer func() {
		if err := session.Close(); err != nil && err != io.EOF {
			log.Logger.Errorf("close ssh session failed: %v", err)
		}
	}()

	res, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(res), err
	}
	return string(res), nil
}

func (c *clientImpl) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
