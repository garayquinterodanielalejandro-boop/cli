package prompter

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/cli/cli/v2/internal/ghinstance"
	"github.com/cli/cli/v2/pkg/surveyext"
	ghPrompter "github.com/cli/go-gh/v2/pkg/prompter"
)

//go:generate moq -rm -out prompter_mock.go . Prompter
type Prompter interface {
	// generic prompts from go-gh
	Select(string, string, []string) (int, error)
	MultiSelect(prompt string, defaults []string, options []string) ([]int, error)
	Input(string, string) (string, error)
	Password(string) (string, error)
	Confirm(string, bool) (bool, error)

	// gh specific prompts
	AuthToken() (string, error)
	ConfirmDeletion(string) error
	InputHostname() (string, error)
	MarkdownEditor(string, string, bool) (string, error)
}

func New(editorCmd string, stdin ghPrompter.FileReader, stdout ghPrompter.FileWriter, stderr ghPrompter.FileWriter) Prompter {
	return &huhPrompter{
		stdin:     stdin,
		stdout:    stdout,
		stderr:    stderr,
		editorCmd: editorCmd,
	}
	// return &surveyPrompter{
	// 	prompter:  ghPrompter.New(stdin, stdout, stderr),
	// 	stdin:     stdin,
	// 	stdout:    stdout,
	// 	stderr:    stderr,
	// 	editorCmd: editorCmd,
	// }
}

type huhPrompter struct {
	stdin     ghPrompter.FileReader
	stdout    ghPrompter.FileWriter
	stderr    ghPrompter.FileWriter
	editorCmd string
}

func (p *huhPrompter) newForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).
		WithTheme(huh.ThemeBase16()).
		WithAccessible(os.Getenv("GH_SCREENREADER_FRIENDLY") != "").
		WithProgramOptions(tea.WithOutput(p.stdout), tea.WithInput(p.stdin))
}

func (p *huhPrompter) Select(prompt, defaultValue string, options []string) (int, error) {
	var result int
	formOptions := []huh.Option[int]{}
	for i, o := range options {
		formOptions = append(formOptions, huh.NewOption(o, i))
	}

	form := p.newForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title(prompt).
				Value(&result).
				Options(formOptions...),
		),
	)

	err := form.Run()
	return result, err
}

func (p *huhPrompter) MultiSelect(prompt string, defaults []string, options []string) ([]int, error) {
	var result []int
	formOptions := make([]huh.Option[int], len(options))
	for i, o := range options {
		formOptions[i] = huh.NewOption(o, i)
	}

	form := p.newForm(
		huh.NewGroup(
			huh.NewMultiSelect[int]().
				Title(prompt).
				Value(&result).
				Limit(len(options)).
				Options(formOptions...),
		),
	)

	if err := form.Run(); err != nil {
		return nil, err
	}

	mid := len(result) / 2
	return result[:mid], nil
}

func (p *huhPrompter) Input(prompt, defaultValue string) (string, error) {
	result := defaultValue
	form := p.newForm(
		huh.NewGroup(
			huh.NewInput().
				Title(prompt).
				Value(&result),
		),
	)

	err := form.Run()
	return result, err
}

func (p *huhPrompter) Password(prompt string) (string, error) {
	var result string
	form := p.newForm(
		huh.NewGroup(
			huh.NewInput().
				Title(prompt).
				Value(&result),
			// This doesn't have any effect in accessible mode.
			// EchoMode(huh.EchoModePassword),
		),
	)

	err := form.Run()
	return result, err
}

func (p *huhPrompter) Confirm(prompt string, _ bool) (bool, error) {
	var result bool
	form := p.newForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(prompt).
				Value(&result),
		),
	)
	if err := form.Run(); err != nil {
		return false, err
	}
	return result, nil
}

func (p *huhPrompter) AuthToken() (string, error) {
	// var result string
	// err := p.ask(&survey.Password{
	// 	Message: "Paste your authentication token:",
	// }, &result, survey.WithValidator(survey.Required))
	// return result, err

	var result string
	form := p.newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Paste your authentication token:").
				Validate(func(input string) error {
					if input == "" {
						return fmt.Errorf("token is required")
					}
					return nil
				}).
				Value(&result),
			// This doesn't have any effect in accessible mode.
			// EchoMode(huh.EchoModePassword),
		),
	)

	err := form.Run()
	return result, err
}

func (p *huhPrompter) ConfirmDeletion(requiredValue string) error {
	// var result string
	// return p.ask(
	// 	&survey.Input{
	// 		Message: fmt.Sprintf("Type %s to confirm deletion:", requiredValue),
	// 	},
	// 	&result,
	// 	survey.WithValidator(
	// 		func(val interface{}) error {
	// 			if str := val.(string); !strings.EqualFold(str, requiredValue) {
	// 				return fmt.Errorf("You entered %s", str)
	// 			}
	// 			return nil
	// 		}))

	var result string
	form := p.newForm(
		huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Type %q to confirm deletion", requiredValue)).
				Validate(func(input string) error {
					if input != requiredValue {
						return fmt.Errorf("You entered: %q", input)
					}
					return nil
				}).
				Value(&result),
			// This doesn't have any effect in accessible mode.
			// EchoMode(huh.EchoModePassword),
		),
	)

	return form.Run()
}

func (p *huhPrompter) InputHostname() (string, error) {
	// var result string
	// err := p.ask(
	// 	&survey.Input{
	// 		Message: "Hostname:",
	// 	}, &result, survey.WithValidator(func(v interface{}) error {
	// 		return ghinstance.HostnameValidator(v.(string))
	// 	}))
	// return result, err

	var result string
	form := p.newForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Hostname:").
				Validate(ghinstance.HostnameValidator).
				Value(&result),
		),
	)

	err := form.Run()
	return result, err
}

func (p *huhPrompter) MarkdownEditor(prompt, defaultValue string, blankAllowed bool) (string, error) {
	// var result string
	// err := p.ask(&surveyext.GhEditor{
	// 	BlankAllowed:  blankAllowed,
	// 	EditorCommand: p.editorCmd,
	// 	Editor: &survey.Editor{
	// 		Message:       prompt,
	// 		Default:       defaultValue,
	// 		FileName:      "*.md",
	// 		HideDefault:   true,
	// 		AppendDefault: true,
	// 	},
	// }, &result)
	// return result, err

	var result string
	form := p.newForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(prompt).
				Options(
					huh.NewOption("Open Editor", "open"),
					huh.NewOption("Skip", "skip"),
				).
				Value(&result),
		),
	)
	if err := form.Run(); err != nil {
		return "", err
	}

	if result == "skip" {
		// TODO: loop if blank not allowed
		if !blankAllowed && defaultValue == "" {
			panic("blank not allowed and no default value")
		}
		return "", nil
	}

	text, err := surveyext.Edit(p.editorCmd, "*.md", defaultValue, p.stdin, p.stdout, p.stderr)
	if err != nil {
		return "", err
	}

	// TODO: blank not allowed
	if !blankAllowed && defaultValue == "" {
		panic("blank not allowed and no default value")
	}

	return text, nil
}

type surveyPrompter struct {
	prompter  *ghPrompter.Prompter
	stdin     ghPrompter.FileReader
	stdout    ghPrompter.FileWriter
	stderr    ghPrompter.FileWriter
	editorCmd string
}

func (p *surveyPrompter) Select(prompt, defaultValue string, options []string) (int, error) {
	return p.prompter.Select(prompt, defaultValue, options)
}

func (p *surveyPrompter) MultiSelect(prompt string, defaultValues, options []string) ([]int, error) {
	return p.prompter.MultiSelect(prompt, defaultValues, options)
}

func (p *surveyPrompter) Input(prompt, defaultValue string) (string, error) {
	return p.prompter.Input(prompt, defaultValue)
}

func (p *surveyPrompter) Password(prompt string) (string, error) {
	return p.prompter.Password(prompt)
}

func (p *surveyPrompter) Confirm(prompt string, defaultValue bool) (bool, error) {
	return p.prompter.Confirm(prompt, defaultValue)
}

func (p *surveyPrompter) AuthToken() (string, error) {
	var result string
	err := p.ask(&survey.Password{
		Message: "Paste your authentication token:",
	}, &result, survey.WithValidator(survey.Required))
	return result, err
}

func (p *surveyPrompter) ConfirmDeletion(requiredValue string) error {
	var result string
	return p.ask(
		&survey.Input{
			Message: fmt.Sprintf("Type %s to confirm deletion:", requiredValue),
		},
		&result,
		survey.WithValidator(
			func(val interface{}) error {
				if str := val.(string); !strings.EqualFold(str, requiredValue) {
					return fmt.Errorf("You entered %s", str)
				}
				return nil
			}))
}

func (p *surveyPrompter) InputHostname() (string, error) {
	var result string
	err := p.ask(
		&survey.Input{
			Message: "Hostname:",
		}, &result, survey.WithValidator(func(v interface{}) error {
			return ghinstance.HostnameValidator(v.(string))
		}))
	return result, err
}

func (p *surveyPrompter) MarkdownEditor(prompt, defaultValue string, blankAllowed bool) (string, error) {
	var result string
	err := p.ask(&surveyext.GhEditor{
		BlankAllowed:  blankAllowed,
		EditorCommand: p.editorCmd,
		Editor: &survey.Editor{
			Message:       prompt,
			Default:       defaultValue,
			FileName:      "*.md",
			HideDefault:   true,
			AppendDefault: true,
		},
	}, &result)
	return result, err
}

func (p *surveyPrompter) ask(q survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	opts = append(opts, survey.WithStdio(p.stdin, p.stdout, p.stderr))
	err := survey.AskOne(q, response, opts...)
	if err == nil {
		return nil
	}
	return fmt.Errorf("could not prompt: %w", err)
}
