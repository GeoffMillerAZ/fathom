package example

import "testing"

func TestZulu(t *testing.T)   {}
func TestAlpha(t *testing.T)  {}
func Testhelper(t *testing.T) {}
func TestWrongSignature()     {}
func ExampleCheck()           {}

type helper struct{}

func (helper) TestMethod(t *testing.T) {}
