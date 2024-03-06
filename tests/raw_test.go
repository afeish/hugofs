//go:build !excludeTest

package tests

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type RawTestSuite struct {
	suite.Suite

	mp string
}

func (s *RawTestSuite) TestCreate() {
	Create(s.mp, s.T())
}
func (s *RawTestSuite) TestWrite() {
	Write(s.mp, s.T())
}
func (s *RawTestSuite) TestReadReg() {
	ReadReg(s.mp, s.T())
}
func (s *RawTestSuite) TestReadDir() {
	ReadDir(s.mp, s.T())
}

func TestRawTestSuite(t *testing.T) {
	v := new(RawTestSuite)
	v.mp = "/tmp/hugo"
	suite.Run(t, v)
}
