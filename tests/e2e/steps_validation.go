//go:build e2e

package e2e

import "github.com/cucumber/godog"

func registerValidationSteps(ctx *godog.ScenarioContext, w *World) {
	ctx.Step(`^"([^"]*)" submits an empty answer "([^"]*)" for question "([^"]*)"$`, w.submitsAnswerForQ)
	ctx.Step(`^"([^"]*)" submits an answer that is not one of the valid options$`, w.submitsInvalidOption)
}

func (w *World) submitsInvalidOption(name string) error {
	qid, err := w.currentQID()
	if err != nil {
		return err
	}
	return w.submitREST(name, qid, "not-a-valid-option")
}
