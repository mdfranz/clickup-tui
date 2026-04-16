package ai

import (
	"context"
	"fmt"
	"os"
	"strings"

	"clickup-tui/pkg/clickup"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/googleai"
)

type Summarizer struct {
	model llms.Model
}

func NewSummarizer() (*Summarizer, error) {
	ctx := context.Background()
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}

	if key == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY environment variable not set")
	}

	opts := []googleai.Option{
		googleai.WithAPIKey(key),
		googleai.WithRest(),
		googleai.WithDefaultModel("gemini-3.1-flash-lite-preview"),
	}

	model, err := googleai.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini model: %v", err)
	}

	return &Summarizer{model: model}, nil
}

func (s *Summarizer) SummarizeTask(task clickup.Task, comments []clickup.Comment) (string, error) {
	ctx := context.Background()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Task Name: %s\n", task.Name))
	b.WriteString(fmt.Sprintf("Status: %s\n", task.Status.Status))
	if task.TextContent != "" {
		b.WriteString(fmt.Sprintf("Description: %s\n", task.TextContent))
	}

	if len(comments) > 0 {
		b.WriteString("\nRecent Comments:\n")
		for i, c := range comments {
			if i >= 5 { // Limit to 5 comments for summary
				break
			}
			b.WriteString(fmt.Sprintf("- %s: %s\n", c.User.Username, strings.TrimSpace(c.CommentText)))
		}
	}

	prompt := fmt.Sprintf(`Please provide a very concise (2-3 sentences) summary of the following ClickUp task based on its name, description, and comments. Focus on the current status and key actions needed.

%s`, b.String())

	res, err := s.model.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %v", err)
	}

	if len(res.Choices) == 0 {
		return "No summary generated.", nil
	}

	return strings.TrimSpace(res.Choices[0].Content), nil
}

func (s *Summarizer) SummarizeTasks(folderName string, tasks []clickup.Task) (string, error) {
	ctx := context.Background()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Folder: %s\n", folderName))
	b.WriteString("Tasks:\n")
	for _, task := range tasks {
		b.WriteString(fmt.Sprintf("- [%s] %s\n", task.Status.Status, task.Name))
		if task.TextContent != "" {
			// Truncate description to keep context window manageable
			desc := task.TextContent
			if len(desc) > 200 {
				desc = desc[:200] + "..."
			}
			b.WriteString(fmt.Sprintf("  Description: %s\n", desc))
		}
	}

	prompt := fmt.Sprintf(`Please provide a high-level summary of the work currently active in the ClickUp folder "%s" based on the following list of tasks. 

Format the response using Markdown with the following structure:
[A brief paragraph summarizing the overall status]

**Overall Progress:**
[Bullet points of key progress items]

**Potential Bottlenecks:**
[Bullet points of potential risks or blockers]

Tasks to consider:
%s`, folderName, b.String())

	res, err := s.model.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate folder summary: %v", err)
	}

	if len(res.Choices) == 0 {
		return "No summary generated.", nil
	}

	return strings.TrimSpace(res.Choices[0].Content), nil
}

