package slsa

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/aquaproj/aqua/v2/pkg/config"
	"github.com/aquaproj/aqua/v2/pkg/cosign"
	"github.com/aquaproj/aqua/v2/pkg/osexec"
	"github.com/aquaproj/aqua/v2/pkg/runtime"
	"github.com/aquaproj/aqua/v2/pkg/timer"
	"github.com/sirupsen/logrus"
)

type CommandExecutor interface {
	ExecStderrAndGetCombinedOutput(cmd *osexec.Cmd) (string, int, error)
}

type Executor interface {
	Verify(ctx context.Context, logE *logrus.Entry, param *ParamVerify, provenancePath string) error
}

type ExecutorImpl struct {
	executor        CommandExecutor
	verifierExePath string
}

func NewExecutor(executor CommandExecutor, param *config.Param) *ExecutorImpl {
	rt := runtime.NewR()
	return &ExecutorImpl{
		executor: executor,
		verifierExePath: ExePath(&ParamExePath{
			RootDir: param.RootDir,
			Runtime: rt,
		}),
	}
}

func wait(ctx context.Context, logE *logrus.Entry, retryCount int) error {
	randGenerator := rand.New(rand.NewSource(time.Now().UnixNano()))       //nolint:gosec
	waitTime := time.Duration(randGenerator.Intn(1000)) * time.Millisecond //nolint:mnd
	logE.WithFields(logrus.Fields{
		"retry_count": retryCount,
		"wait_time":   waitTime,
	}).Info("Verification by slsa-verifier failed temporarily, retrying")
	if err := timer.Wait(ctx, waitTime); err != nil {
		return fmt.Errorf("wait running slsa-verifier: %w", err)
	}
	return nil
}

func (e *ExecutorImpl) exec(ctx context.Context, args []string) (string, error) {
	mutex := cosign.GetMutex()
	mutex.Lock()
	defer mutex.Unlock()
	out, _, err := e.executor.ExecStderrAndGetCombinedOutput(osexec.Command(ctx, e.verifierExePath, args...))
	return out, err //nolint:wrapcheck
}

var errVerify = errors.New("verify with slsa-verifier")

func (e *ExecutorImpl) Verify(ctx context.Context, logE *logrus.Entry, param *ParamVerify, provenancePath string) error {
	if param.SourceTag == "" {
		return errors.New("source tag is empty")
	}
	args := []string{
		"verify-artifact",
		param.ArtifactPath,
		"--provenance-path",
		provenancePath,
		"--source-uri",
		param.SourceURI,
	}
	if param.SourceTag != "-" {
		args = append(args, "--source-tag", param.SourceTag)
	}
	for i := range 5 {
		if _, err := e.exec(ctx, args); err == nil {
			return nil
		}
		if i == 4 { //nolint:mnd
			break
		}
		if err := wait(ctx, logE, i+1); err != nil {
			return err
		}
	}
	return errVerify
}
