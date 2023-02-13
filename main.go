package main

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"fmt"
	"os"
	"strings"

	gpt3 "github.com/sashabaranov/go-gpt3"
	"github.com/spf13/cobra"
)

var LongHelp = `
Chat with ChatGPT in console.

Examples:
  # start an interactive session
  chatgpt -i

  # ask chatgpt for a one-time response
  chatgpt -q "answer me this ChatGPT..."

  # provide context to a question or conversation
  chatgpt context.txt -i
  chatgpt context.txt -q "answer me this ChatGPT..."

  # read context from file and write response back
  chatgpt convo.txt

  # pipe content from another program, useful for ! in vim visual mode
  cat convo.txt | chatgpt

  # inspect the predifined pretexts, which set ChatGPT's mood
  chatgpt -p list
  chatgpt -p view:<name>

  # use a pretext with any of the previous modes
  chatgpt -p optimistic -i
  chatgpt -p cynic -q "Is the world going to be ok?"
  chatgpt -p teacher convo.txt
`

//go:embed pretexts/*
var predefined embed.FS

var Question string
var Pretext string
var MaxTokens int
var PromptMode bool
var PromptText string

func GetResponse(client *gpt3.Client, ctx context.Context, question string) (string, error) {
	req := gpt3.CompletionRequest{
		Model:     gpt3.GPT3TextDavinci003,
		MaxTokens: MaxTokens,
		Prompt:    question,
	}
	resp, err := client.CreateCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Text, nil
}

type NullWriter int

func (NullWriter) Write([]byte) (int, error) { return 0, nil }

func main() {
	apiKey := os.Getenv("CHATGPT_API_KEY")
	if apiKey == "" {
		fmt.Println("CHATGPT_API_KEY environment var is missing\nVisit https://platform.openai.com/account/api-keys to get one\n")
		os.Exit(1)
	}

	client := gpt3.NewClient(apiKey)

	rootCmd := &cobra.Command{
		Use:   "chatgpt [file]",
		Short: "Chat with ChatGPT in console.",
		Long: LongHelp,
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			var filename string

			if Pretext != "" {

				files, err := predefined.ReadDir("pretexts")
				if err != nil {
					panic(err)
				}

				if Pretext == "list" {
					for _, f := range files {
						fmt.Println(strings.TrimSuffix(f.Name(), ".txt"))
					}
					os.Exit(0)
				}

				if strings.HasPrefix(Pretext, "view:") {
					name := strings.TrimPrefix(Pretext, "view:")
					contents, err := predefined.ReadFile("pretexts/" + name + ".txt")
					if err != nil {
						fmt.Println(err)
						os.Exit(1)
					}
					fmt.Println(string(contents))
					os.Exit(0)
				}

				// look for predefined
				for _, f := range files {
					name := strings.TrimSuffix(f.Name(), ".txt")
					if name == Pretext {
						contents, err := predefined.ReadFile("pretexts/" + name + ".txt")
						if err != nil {
							fmt.Println(err)
							os.Exit(1)
						}
						PromptText = string(contents)
						break
					}
				}

				if PromptText == "" {
					PromptText = Pretext
				}

			}

			if len(args) == 0 && !PromptMode && Question == "" {
				reader := bufio.NewReader(os.Stdin)
				var buf bytes.Buffer
				for {
						b, err := reader.ReadByte()
						if err != nil {
								break
						}
						buf.WriteByte(b)
				}
				PromptText += buf.String()
			} else if len(args) == 1 {
				filename = args[0]
				content, err := os.ReadFile(filename)
				if err != nil {
					fmt.Println(err)
					return
				}
				PromptText += string(content)
			}

			if Question != "" {
				PromptText += "\n" + Question
			}

			if PromptMode {
				fmt.Println(PromptText)
				err = RunPrompt(client)
			} else {
				err = RunOnce(client, filename)
			}

			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}

		},
	}


	rootCmd.Flags().StringVarP(&Question, "question", "q", "", "ask a single question and print the response back")
	rootCmd.Flags().StringVarP(&Pretext, "pretext", "p", "", "pretext to add to ChatGPT input, use 'list' or 'view:<name>' to inspect predefined, '<name>' to use a pretext, or otherwise supply any custom text")
	rootCmd.Flags().BoolVarP(&PromptMode, "interactive", "i", false, "start an interactive session with ChatGPT")
	rootCmd.Flags().IntVarP(&MaxTokens, "tokens", "t", 420, "set the MaxTokens to generate per response")

	rootCmd.Execute()
}

func RunPrompt(client *gpt3.Client) error {
	ctx := context.Background()
	scanner := bufio.NewScanner(os.Stdin)
	quit := false

	for !quit {
		fmt.Print("> ")

		if !scanner.Scan() {
			break
		}

		question := scanner.Text()
		switch question {
		case "quit", "q", "exit":
			quit = true

		default:
			PromptText += "\n\n> " + question + "\n"
			r, err := GetResponse(client, ctx, PromptText)
			if err != nil {
				return err
			}

			PromptText += "\n" + r + "\n"
			fmt.Println(r + "\n")
		}
	}
	
	return nil
}

func RunOnce(client *gpt3.Client, filename string) error {
	ctx := context.Background()

	r, err := GetResponse(client, ctx, PromptText)
	if err != nil {
		return err
	}

	if filename == "" {
		fmt.Println(r)
	} else {
		err = AppendToFile(filename, r)
		if err != nil {
			return err
		}
	}

	return nil
}

// AppendToFile provides a function to append data to an existing file,
// creating it if it doesn't exist
func AppendToFile(filename string, data string) error {
	// Open the file in append mode
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// Append the data to the file
	_, err = file.WriteString(data)
	if err != nil {
		return err
	}

	return file.Close()
}