package completions

import (
	"encoding/base64"
	"fmt"
	"slices"
	"time"

	"github.com/nanobot-ai/nanobot/pkg/mcp"
	"github.com/nanobot-ai/nanobot/pkg/types"
)

func toResponse(resp *Response, created time.Time) (*types.CompletionResponse, error) {
	result := &types.CompletionResponse{
		Model: resp.Model,
		Output: types.Message{
			ID:      resp.ID,
			Created: &created,
			Role:    "assistant",
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if choice.Message != nil {
			// Handle reasoning (for reasoning models)
			if choice.Message.Reasoning != nil && *choice.Message.Reasoning != "" {
				result.Output.Items = append(result.Output.Items, types.CompletionItem{
					ID: fmt.Sprintf("%s-reasoning", resp.ID),
					Reasoning: &types.Reasoning{
						Summary: []types.SummaryText{
							{
								Text: *choice.Message.Reasoning,
							},
						},
					},
				})
			}

			// Handle content
			if choice.Message.Content.Text != nil {
				result.Output.Items = append(result.Output.Items, types.CompletionItem{
					ID: fmt.Sprintf("%s-content", resp.ID),
					Content: &mcp.Content{
						Type: "text",
						Text: *choice.Message.Content.Text,
					},
				})
			}

			// Handle tool calls
			for i, toolCall := range choice.Message.ToolCalls {
				result.Output.Items = append(result.Output.Items, types.CompletionItem{
					ID: fmt.Sprintf("%s-%d", resp.ID, i),
					ToolCall: &types.ToolCall{
						CallID:    toolCall.ID,
						Name:      toolCall.Function.Name,
						Arguments: toolCall.Function.Arguments,
					},
				})
			}

			// Handle refusal
			if choice.Message.Refusal != nil {
				result.Output.Items = append(result.Output.Items, types.CompletionItem{
					ID: fmt.Sprintf("%s-refusal", resp.ID),
					Content: &mcp.Content{
						Type: "text",
						Text: "REFUSAL: " + *choice.Message.Refusal,
					},
				})
			}
		}
	}

	return result, nil
}

func toRequest(req *types.CompletionRequest) (Request, error) {
	if req.MaxTokens == 0 {
		req.MaxTokens = 4096
	}

	result := Request{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Metadata:    req.Metadata,
	}

	// Set max tokens (use max_completion_tokens for newer models)
	result.MaxCompletionTokens = &req.MaxTokens

	// Handle tools
	for _, tool := range req.Tools {
		result.Tools = append(result.Tools, Tool{
			Type: "function",
			Function: Function{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	// Handle tool choice
	if req.ToolChoice != "" {
		switch req.ToolChoice {
		case "auto":
			result.ToolChoice = &ToolChoice{Type: "auto"}
		case "none":
			result.ToolChoice = &ToolChoice{Type: "none"}
		case "required":
			result.ToolChoice = &ToolChoice{Type: "required"}
		default:
			// Specific tool name
			result.ToolChoice = &ToolChoice{
				Type: "function",
				Function: &ToolChoiceFunc{
					Name: req.ToolChoice,
				},
			}
		}
	}

	// Handle output schema
	if req.OutputSchema != nil {
		result.ResponseFormat = &ResponseFormat{
			Type: "json_schema",
			JSONSchema: &JSONSchema{
				Name:        req.OutputSchema.Name,
				Description: req.OutputSchema.Description,
				Schema:      req.OutputSchema.ToSchema(),
				Strict:      req.OutputSchema.Strict,
			},
		}
		if result.ResponseFormat.JSONSchema.Name == "" {
			result.ResponseFormat.JSONSchema.Name = "output-schema"
		}
	}

	// Convert messages
	for _, msg := range req.Input {
		openAIMsg := Message{
			Role: msg.Role,
		}

		// Image data URLs returned by tool calls in this message. The OpenAI
		// chat/completions schema only allows image_url parts inside user
		// messages, so we collect them here and emit a follow-up user message
		// after the tool message below.
		var toolResultImageURLs []string

		// Handle single text content case
		if len(msg.Items) == 1 && msg.Items[0].Content != nil && msg.Items[0].Content.Type == "text" && msg.Items[0].Content.Text != "" {
			openAIMsg.Content.Text = &msg.Items[0].Content.Text
		} else {
			// Handle multi-part content
			var parts []ContentPart
			for _, item := range msg.Items {
				if item.Content != nil {
					switch item.Content.Type {
					case "text":
						// Skip empty text content
						if item.Content.Text != "" {
							parts = append(parts, ContentPart{
								Type: "text",
								Text: item.Content.Text,
							})
						}
					case "image":
						parts = append(parts, ContentPart{
							Type: "image_url",
							ImageURL: &ImageURL{
								URL:    item.Content.ToImageURL(),
								Detail: "auto",
							},
						})
					case "resource":
						if item.Content.Resource != nil && item.Content.Resource.Annotations != nil && slices.Contains(item.Content.Resource.Annotations.Audience, "assistant") {
							if _, ok := types.ImageMimeTypes[item.Content.Resource.MIMEType]; ok {
								url := fmt.Sprintf("data:%s;base64,%s", item.Content.Resource.MIMEType, item.Content.Resource.Blob)
								parts = append(parts, ContentPart{
									Type: "image_url",
									ImageURL: &ImageURL{
										URL:    url,
										Detail: "auto",
									},
								})
							} else if _, ok := types.TextMimeTypes[item.Content.Resource.MIMEType]; ok {
								text := item.Content.Resource.Text
								if item.Content.Resource.Blob != "" {
									bytes, _ := base64.StdEncoding.DecodeString(item.Content.Resource.Blob)
									text = string(bytes)
								}
								parts = append(parts, ContentPart{
									Type: "text",
									Text: text,
								})
							} else if _, ok := types.PDFMimeTypes[item.Content.Resource.MIMEType]; ok {
								// For OpenAI completions API, PDFs are not directly supported like in anthropic
								// Convert to text representation or skip
								text := fmt.Sprintf("[PDF Document: %s]", item.Content.Resource.URI)
								if item.Content.Resource.Text != "" {
									text = item.Content.Resource.Text
								}
								parts = append(parts, ContentPart{
									Type: "text",
									Text: text,
								})
							}
						}
					}
				} else if item.ToolCall != nil {
					// Handle tool calls (assistant message)
					openAIMsg.ToolCalls = append(openAIMsg.ToolCalls, ToolCall{
						ID:   item.ToolCall.CallID,
						Type: "function",
						Function: FunctionCall{
							Name:      item.ToolCall.Name,
							Arguments: item.ToolCall.Arguments,
						},
					})
				} else if item.ToolCallResult != nil {
					// Handle tool call results (tool message)
					openAIMsg.Role = "tool"
					openAIMsg.ToolCallID = item.ToolCallResult.CallID

					// Combine all content into text. Image content is captured
					// separately and emitted as a follow-up user message below,
					// since chat/completions disallows image_url inside tool
					// messages.
					var resultText string
					for _, content := range item.ToolCallResult.Output.Content {
						if content.Type == "text" {
							if resultText != "" {
								resultText += "\n"
							}
							resultText += content.Text
						} else if content.Type == "image" && content.Data != "" && content.MIMEType != "" {
							toolResultImageURLs = append(toolResultImageURLs, content.ToImageURL())
							if resultText != "" {
								resultText += "\n"
							}
							resultText += "[Image attached; see following message]"
						} else if content.Type == "resource" && content.Resource != nil && content.Resource.Annotations != nil && slices.Contains(content.Resource.Annotations.Audience, "assistant") {
							if _, ok := types.TextMimeTypes[content.Resource.MIMEType]; ok {
								text := content.Resource.Text
								if content.Resource.Blob != "" {
									bytes, _ := base64.StdEncoding.DecodeString(content.Resource.Blob)
									text = string(bytes)
								}
								if resultText != "" {
									resultText += "\n"
								}
								resultText += text
							} else if _, ok := types.ImageMimeTypes[content.Resource.MIMEType]; ok {
								if content.Resource.Blob != "" {
									url := fmt.Sprintf("data:%s;base64,%s", content.Resource.MIMEType, content.Resource.Blob)
									toolResultImageURLs = append(toolResultImageURLs, url)
								}
								if resultText != "" {
									resultText += "\n"
								}
								if content.Resource.URI != "" {
									resultText += fmt.Sprintf("[Image: %s]", content.Resource.URI)
								} else {
									resultText += "[Image attached; see following message]"
								}
							} else if _, ok := types.PDFMimeTypes[content.Resource.MIMEType]; ok {
								if resultText != "" {
									resultText += "\n"
								}
								if content.Resource.Text != "" {
									resultText += content.Resource.Text
								} else {
									resultText += fmt.Sprintf("[PDF Document: %s]", content.Resource.URI)
								}
							}
						}
					}

					if resultText == "" {
						resultText = "Tool execution completed"
					}

					openAIMsg.Content.Text = &resultText
				}
			}

			if len(parts) > 0 {
				openAIMsg.Content.ContentParts = parts
			}
		}

		result.Messages = append(result.Messages, openAIMsg)

		// If the tool result contained images, emit a follow-up user message
		// carrying them as image_url parts. Without this, vision-capable
		// chat/completions backends never receive the image bytes from a
		// tool call.
		if len(toolResultImageURLs) > 0 {
			followupParts := []ContentPart{
				{Type: "text", Text: "Images returned by the previous tool call:"},
			}
			for _, url := range toolResultImageURLs {
				followupParts = append(followupParts, ContentPart{
					Type: "image_url",
					ImageURL: &ImageURL{
						URL:    url,
						Detail: "auto",
					},
				})
			}
			result.Messages = append(result.Messages, Message{
				Role: "user",
				Content: MessageContent{
					ContentParts: followupParts,
				},
			})
		}
	}

	// Add system message if present
	if req.SystemPrompt != "" {
		systemMsg := Message{
			Role: "system",
			Content: MessageContent{
				Text: &req.SystemPrompt,
			},
		}
		// Prepend system message
		result.Messages = append([]Message{systemMsg}, result.Messages...)
	}

	return result, nil
}
