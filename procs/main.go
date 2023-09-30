package procs

import (
	"bufio"
	"fmt"
	"github.com/JonasBak/autoscaler-proxy/utils"
	"io"
	"os/exec"
	"sync"
	"syscall"
)

var log = utils.Logger().WithField("pkg", "procs")

type ProcsOpts struct {
	Run []string          `yaml:"run"`
	Env map[string]string `yaml:"env"`
}

type process struct {
	p      *exec.Cmd
	rawCmd string
}

type Procs struct {
	procs []process
	wg    *sync.WaitGroup
}

func New(opts ProcsOpts) Procs {
	procs := []process{}

	variables := make(map[string]string)
	env := utils.BuildTemplate(utils.WithEnvMap(utils.TemplateMap(variables)), opts.Env).(map[string]string)

	envList := []string{}

	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	for _, cmd := range opts.Run {
		p := exec.Command("/bin/sh", "-c", cmd)
		p.Env = envList
		procs = append(procs, process{p: p, rawCmd: cmd})
	}

	return Procs{
		procs: procs,
		wg:    &sync.WaitGroup{},
	}
}

func (p Procs) Run() {
	for _, proc := range p.procs {
		proc := proc

		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			log := log.WithField("cmd", proc.rawCmd)

			stdout, err := proc.p.StdoutPipe()
			if err != nil {
				log.WithError(err).Warn("failed to get stdout")
				return
			}
			go func() {
				log := log.WithField("output", "stdout")
				err = logOutput(stdout, func(line string) {
					log.Info(line)
				})
				if err != nil {
					log.WithError(err).Warn("failed to read command output")
				}
			}()
			stderr, err := proc.p.StderrPipe()
			if err != nil {
				log.WithError(err).Warn("failed to get stderr")
				return
			}
			go func() {
				log := log.WithField("output", "stderr")
				err = logOutput(stderr, func(line string) {
					log.Warn(line)
				})
				if err != nil {
					log.WithError(err).Warn("failed to read command output")
				}
			}()

			log.Info("running command")
			err = proc.p.Run()
			if err != nil {
				log.WithError(err).Warn("command exited with error")
			}
		}()
	}
}

func (p Procs) Shutdown() {
	go func() {
		for _, proc := range p.procs {
			proc.p.Process.Signal(syscall.SIGTERM)
		}
	}()
	p.wg.Wait()
}

func (p Procs) Kill() {
	for _, proc := range p.procs {
		proc.p.Process.Kill()
	}
}

func logOutput(r io.Reader, logFunc func(string)) error {
	reader := bufio.NewReader(r)

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF || err == io.ErrClosedPipe {
			return nil
		} else if err != nil {
			return err
		}
		logFunc(line)
	}
}
