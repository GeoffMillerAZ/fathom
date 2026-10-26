package hops

import "example.test/hops/leaf"

func Entry() { Middle() }

func Middle() { leaf.Gold() }

func Unreachable() {}

func Cycle() { Cycle(); Entry() }

type Runner interface{ Run() }

type Worker struct{}

func (Worker) Run() { leaf.Gold() }

func Dispatch(r Runner) { r.Run() }
