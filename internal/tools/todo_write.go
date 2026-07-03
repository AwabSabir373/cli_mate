package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type TodoItem struct {
	ID        int    `json:"id"`
	Task      string `json:"task"`
	Completed bool   `json:"completed"`
}

type TodoWriteTool struct {
	Todos []TodoItem
}

func NewTodoWriteTool() *TodoWriteTool {
	return &TodoWriteTool{}
}

func (t *TodoWriteTool) Name() string {
	return "todo_write"
}

func (t *TodoWriteTool) Definition() Definition {
	return Definition{
		Name:        t.Name(),
		Description: "Manage a task checklist for tracking progress during multi-step work.",
		Schema: map[string]any{
			"type":     "object",
			"required": []string{"todos"},
			"properties": map[string]any{
				"todos": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id": map[string]any{
								"type":        "integer",
								"description": "Unique identifier for the todo item",
							},
							"task": map[string]any{
								"type":        "string",
								"description": "Description of the task",
							},
							"completed": map[string]any{
								"type":        "boolean",
								"description": "Whether the task is completed",
							},
						},
						"required": []string{"task", "completed"},
					},
					"description": "The complete list of todos to set.",
				},
			},
			"description": "Update the task checklist.",
		},
	}
}

func (t *TodoWriteTool) Execute(_ context.Context, call Call) (Result, error) {
	todosRaw, ok := call.Argument["todos"]
	if !ok {
		return Result{Error: "todos is required"}, fmt.Errorf("todos is required")
	}

	todosJSON, err := json.Marshal(todosRaw)
	if err != nil {
		return Result{Error: "invalid todos format"}, fmt.Errorf("invalid todos format: %w", err)
	}

	var newTodos []TodoItem
	if err := json.Unmarshal(todosJSON, &newTodos); err != nil {
		return Result{Error: "invalid todos structure"}, fmt.Errorf("invalid todos structure: %w", err)
	}

	for i := range newTodos {
		if newTodos[i].ID == 0 {
			newTodos[i].ID = i + 1
		}
	}

	allCompleted := true
	for _, todo := range newTodos {
		if !todo.Completed {
			allCompleted = false
			break
		}
	}

	var output string
	if allCompleted && len(newTodos) > 0 {
		t.Todos = nil
		output = "All tasks completed! Todo list has been reset."
	} else {
		t.Todos = newTodos
		output = formatTodoList(newTodos)
	}

	return Result{Content: output}, nil
}

func (t *TodoWriteTool) GetTodos() []TodoItem {
	return t.Todos
}

func formatTodoList(todos []TodoItem) string {
	if len(todos) == 0 {
		return "No tasks in the todo list."
	}

	var b strings.Builder
	b.WriteString("Task Checklist:\n\n")
	for _, todo := range todos {
		if todo.Completed {
			fmt.Fprintf(&b, "[x] %s\n", todo.Task)
		} else {
			fmt.Fprintf(&b, "[ ] %s\n", todo.Task)
		}
	}
	return b.String()
}
