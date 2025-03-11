package main

import (
	"fmt"
	"log"

	"github.com/cli/cli/v2/internal/prompter"
	"github.com/cli/cli/v2/pkg/iostreams"
)

func main() {
	io := iostreams.System()
	p := prompter.New("vim", io.In, io.Out, io.ErrOut)

	fmt.Println("Demonstrating Single Select")
	cuisines := []string{"Italian", "Greek", "Indian", "Japanese", "American"}
	favorite, err := p.Select("Favorite cuisine?", "Italian", cuisines)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Favorite cuisine: %s\n", cuisines[favorite])

	fmt.Println("Demonstrating Multi Select")
	favorites, err := p.MultiSelect("Favorite cuisines?", []string{}, cuisines)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range favorites {
		fmt.Printf("Favorite cuisine: %s\n", cuisines[f])
	}

	fmt.Println("Demonstrating Text Input")
	text, err := p.Input("Favorite meal?", "Breakfast")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Favorite meal: %s\n", text)

	fmt.Println("Demonstrating Password Input (unmasked)")
	safeword, err := p.Password("Safe word?")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Safe word: %s\n", safeword)

	fmt.Println("Demonstrating Confirmation")
	confirmation, err := p.Confirm("Are you sure?", false)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Confirmation: %t\n", confirmation)

	fmt.Println("Demonstrating Auth Token (can't be blank)")
	token, err := p.AuthToken()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Auth token: %s\n", token)

	fmt.Println("Demonstrating Deletion Confirmation")
	err = p.ConfirmDeletion("delete-me")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Item deleted")

	fmt.Println("Demonstrating Hostname (must be valid hostname)")
	hostname, err := p.InputHostname()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Hostname: %s\n", hostname)

	fmt.Println("Demonstrating Markdown Editor with Initial Text")
	editorText, err := p.MarkdownEditor("Edit your text:", "Initial text", true)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Edited text: %s\n", editorText)
}
