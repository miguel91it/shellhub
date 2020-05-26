// +build !docker

package sshd

import (
	"os/exec"
	"strconv"
	"syscall"

	"github.com/shellhub-io/shellhub/agent/pkg/osauth"
)

func newCmd(u *osauth.User, shell, term, host string, command ...string) *exec.Cmd {
	uid, _ := strconv.Atoi(u.UID)
	gid, _ := strconv.Atoi(u.GID)

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = []string{
		"TERM=" + term,
		"HOME=" + u.HomeDir,
		"SHELL=" + shell,
		"SHELLHUB_HOST=" + host,
	}
	cmd.Dir = u.HomeDir
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	return cmd
}
