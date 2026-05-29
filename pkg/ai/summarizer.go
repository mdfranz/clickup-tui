package ai

import (
	"context"
	"fmt"
	"os"
	"strings"

	"clickup-tui/pkg/clickup"
	"clickup-tui/pkg/format"

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

func (s *Summarizer) SummarizeUserActivity(userName string, date string, activities []clickup.Activity, taskDetails map[string]clickup.Task, taskComments map[string][]clickup.Comment) (string, error) {
	ctx := context.Background()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("User: %s\n", userName))
	b.WriteString(fmt.Sprintf("Date: %s\n\n", date))

	// Group activities by task to provide better context
	taskActivities := make(map[string][]clickup.Activity)
	for _, a := range activities {
		taskActivities[a.TaskID] = append(taskActivities[a.TaskID], a)
	}

	for taskID, acts := range taskActivities {
		task, hasTask := taskDetails[taskID]
		if hasTask {
			b.WriteString(fmt.Sprintf("Task: [%s] %s\n", task.Status.Status, task.Name))
			if task.TextContent != "" {
				desc := task.TextContent
				if len(desc) > 300 {
					desc = desc[:300] + "..."
				}
				b.WriteString(fmt.Sprintf("  Description: %s\n", strings.ReplaceAll(desc, "\n", " ")))
			}
		} else {
			b.WriteString(fmt.Sprintf("Task ID: %s (Details unavailable)\n", taskID))
		}

		b.WriteString("  User Actions:\n")
		for _, a := range acts {
			b.WriteString(fmt.Sprintf("  - %s\n", a.Type))
		}

		comments, hasComments := taskComments[taskID]
		if hasComments && len(comments) > 0 {
			b.WriteString("  User Comments on this task:\n")
			for _, c := range comments {
				if c.User.Username == userName { // Only show this user's comments for context
					commentText := strings.TrimSpace(c.CommentText)
					if len(commentText) > 200 {
						commentText = commentText[:200] + "..."
					}
					b.WriteString(fmt.Sprintf("  - \"%s\"\n", strings.ReplaceAll(commentText, "\n", " ")))
				}
			}
		}
		b.WriteString("\n")
	}

	prompt := fmt.Sprintf(`Please provide a concise narrative summary of %s's activity on %s based on the provided actions and comments.
Focus on what was accomplished, what was discussed, and the overall progress made.
Do not just list the tasks; weave them into a coherent 1-2 paragraph summary of the day's work.

Activity Data:
%s`, userName, date, b.String())

	res, err := s.model.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate activity summary: %v", err)
	}

	if len(res.Choices) == 0 {
		return "No summary generated.", nil
	}

	return strings.TrimSpace(res.Choices[0].Content), nil
}

func (s *Summarizer) SummarizeTeamActivity(days int, userActivities map[string][]clickup.Activity, taskDetails map[string]clickup.Task) (string, error) {
	ctx := context.Background()

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Team Activity Report for the last %d Days\n\n", days))

	for username, activities := range userActivities {
		b.WriteString(fmt.Sprintf("### Member: %s\n", username))

		// Group by task for better readability
		taskActivities := make(map[string][]clickup.Activity)
		for _, a := range activities {
			taskActivities[a.TaskID] = append(taskActivities[a.TaskID], a)
		}

		for taskID, acts := range taskActivities {
			task, hasTask := taskDetails[taskID]
			if hasTask {
				b.WriteString(fmt.Sprintf("- Task: [%s] %s (ID: %s)\n", task.Status.Status, task.Name, task.ID))
			} else {
				b.WriteString(fmt.Sprintf("- Task ID: %s (Details unavailable)\n", taskID))
			}
			for _, a := range acts {
				dateStr := format.FormatCommentDate(a.Date)
				if dateStr != "" {
					b.WriteString(fmt.Sprintf("  - [%s] %s\n", dateStr, a.Type))
				} else {
					b.WriteString(fmt.Sprintf("  - %s\n", a.Type))
				}
				if a.Detail != "" {
					b.WriteString(fmt.Sprintf("    Detail: %s\n", strings.ReplaceAll(a.Detail, "\n", " ")))
				}
			}
		}
		b.WriteString("\n")
	}

	prompt := fmt.Sprintf(`Analyze the following ClickUp activity logs for the team over the last %d days and generate a factual, objective, and concise **Team Activity Summary** in Markdown.

Strict Guidelines:
1. Do not use hyperbolic, grandiose, or embellished language. Avoid adjectives like "instrumental," "vital," "significant," "collaboration has been high," etc.
2. Be strictly factual and base everything directly on the logs.
3. Keep sentences short and to the point.
4. Do not use emojis in headers.
5. Capture and include specific dates/times (e.g., "on 05/29") when describing when specific tasks were completed, updated, or commented on, based directly on the timestamp bracketed in the logs.
6. Include activities and comments if they are provided. Indicate when no details have been provided on state changes.

Format the summary with the following structure:

# Team Status Report (Last %d Days)

Provide a 2-3 sentence summary of the period of review.

## Key Achievements & Completed Work
- List specific tasks that were completed or closed in the last %d days based directly on the logs. Keep description of achievements factual and objective.

## Individual Activity & Progress
For each active team member, provide a concise bulleted list or a direct, factual 1-2 sentence summary of the specific tasks they created, updated, or commented on. Do not embellish their role or impact.
Format:
- **[Member Name]**: [Factual summary of what tasks they updated, created, or commented on]

## Discussions & Comments
- Summarize specific key points discussed in task comments based on the log (e.g., vendor meetings, SOC2 collection, groups). If no relevant discussion, state "No discussion logs available."

## Blockers, Risks & Friction
- Factual list of tasks currently blocked or showing delays, with the reported reason. If none, state "No blockers reported."
- Capture open/in progress/blocked tasks that have not been updated during the review period that may need attention.

Team Activity Logs:
%s`, days, days, days, b.String())

	res, err := s.model.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate team activity summary: %v", err)
	}

	if len(res.Choices) == 0 {
		return "No summary generated.", nil
	}

	return strings.TrimSpace(res.Choices[0].Content), nil
}
