package tester

import (
	"github.com/Iori372552686/GoOne/tools/tester/tester_util"
	"testing"
)

func TestLogin(t *testing.T) {
	s := tester_util.NewSession(t)
	err := s.OpenAndLogin()
	if err != nil {
		return
	}
	defer s.LogoutAndClose()
}
