package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

const (
	summarizePrompt = `
        您是一位电影/剧集专家。
		请分析提供的字幕样本和文件名，总结电影/剧集的背景、类型、主要主题，
		以及有助于准确翻译的上下文信息。请尽量简洁（2-3句话）。
        `
	translatePrompt = `
		您是一位专业的电影/电视剧字幕翻译。我将提供一个长度为 %d 的英文字幕 JSON 数组。
		您的任务是根据上下文将数组中每一项翻译成中文。数组是电影中时间相近的对话，翻译时请考虑上下文。
		
		重要提示：翻译完成后，您必须调用 "submit_translation" 函数提交您的翻译，而不是直接输出。
		该函数会检查翻译后的数组长度是否与翻译前相同。如果验证失败，您必须更正翻译并重试。
		
		关键要求：
		
		1. 每个输入字幕必须恰好对应一条输出翻译
		2. 请勿添加或删除任何数组元素
		3. 始终使用 "submit_translation" 函数检查您的翻译
		4. 如果验证失败，请更正翻译并重试

		预期数组长度：%d
		`
	translateWithContextPrompt = `
		您是一位专业的电影/电视剧字幕翻译。我将提供一个长度为 %d 的英文字幕 JSON 数组。
		您的任务是根据上下文将数组中每一项翻译成中文。数组是电影中时间相近的对话，翻译时请考虑上下文。
		
		重要提示：翻译完成后，您必须调用 "submit_translation" 函数提交您的翻译，而不是直接输出。
		该函数会检查翻译后的数组长度是否与翻译前相同。如果验证失败，您必须更正翻译并重试。
		
		关键要求：
		
		1. 每个输入字幕必须恰好对应一条输出翻译
		2. 请勿添加或删除任何数组元素
		3. 始终使用 "submit_translation" 函数检查您的翻译
		4. 如果验证失败，请更正翻译并重试

		预期数组长度：%d
		
		电影/电视剧上下文: %s
		`
)

type Translator struct {
	model   model.ToolCallingChatModel
	context string
}

type SubmitTranslationUserdata struct {
	ExpectedCount int
}

var submitTranslationUserdataKey = SubmitTranslationUserdata{}

type SubmitTranslationReq struct {
	Translations []string `json:"translations"`
}

type SubmitTranslationResp struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason"`
}

type SubmitTranslationTool struct{}

func (t *SubmitTranslationTool) Info() *schema.ToolInfo {
	return &schema.ToolInfo{
		Name: "submit_translation",
		Desc: "提交翻译结果。该函数会检查翻译后的数组长度是否与输入数组长度一致。如果不一致，返回错误信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"translations": {
				Type:     schema.Array,
				Required: true,
				Desc:     "翻译后的文本数组",
				ElemInfo: &schema.ParameterInfo{
					Type: schema.String,
				},
			},
		}),
	}
}

func (t *SubmitTranslationTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	log.Printf("[分组翻译] 提交翻译结果")
	log.Printf("[分组翻译] 输入: %s", argumentsInJSON)
	var input SubmitTranslationReq
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		log.Printf("[分组翻译] 输入解析失败: %v", err)
		return "", fmt.Errorf("invalid input: %w", err)
	}

	expectedCount := ctx.Value(submitTranslationUserdataKey).(*SubmitTranslationUserdata).ExpectedCount

	if len(input.Translations) != expectedCount {
		output := SubmitTranslationResp{
			Valid:  false,
			Reason: fmt.Sprintf("翻译后数组长度 %d 与翻译前数组长度 %d 不匹配！请重新翻译，确保数组中每一个输入都对应一个输出。", len(input.Translations), expectedCount),
		}
		result, _ := json.Marshal(output)
		log.Printf("[分组翻译] 输出: %s", string(result))
		return string(result), nil
	}

	output := SubmitTranslationResp{
		Valid:  true,
		Reason: "正确",
	}
	result, _ := json.Marshal(output)
	log.Printf("[分组翻译] 输出: %s", string(result))
	return string(result), nil
}

func NewTranslator(ctx context.Context, apiKey string, baseURL string, modelName string) (*Translator, error) {
	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey:  apiKey,
		Model:   modelName,
		BaseURL: baseURL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chat model: %w", err)
	}

	validateTool := &SubmitTranslationTool{}

	toolModel, err := chatModel.WithTools([]*schema.ToolInfo{
		validateTool.Info(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to bind tools: %w", err)
	}

	return &Translator{
		model:   toolModel,
		context: "",
	}, nil
}

func (t *Translator) SummarizeContext(ctx context.Context, filename string, subtitles []*SubtitleItem) error {
	log.Printf("[背景信息] 开始总结电影背景信息")

	sampleText := ""
	maxChars := 20000
	for _, item := range subtitles {
		if len(sampleText)+len(item.Text) > maxChars {
			break
		}
		if sampleText != "" {
			sampleText += "\n"
		}
		sampleText += item.Text
	}

	messages := []*schema.Message{
		schema.SystemMessage(summarizePrompt),
		schema.UserMessage(fmt.Sprintf("Filename: %s\n\nSubtitle samples:\n%s", filename, sampleText)),
	}

	resp, err := t.model.Generate(ctx, messages)
	if err != nil {
		log.Printf("[背景信息] 总结失败: %v", err)
		return fmt.Errorf("failed to summarize context: %w", err)
	}

	t.context = string(resp.Content)
	log.Printf("[背景信息] 背景信息总结: %s", t.context)
	return nil
}

type SubtitleGroup struct {
	Indices []int
	Texts   []string
}

func GroupSubtitlesByTime(items []*SubtitleItem, maxGapSeconds float64) []SubtitleGroup {
	if len(items) == 0 {
		return []SubtitleGroup{}
	}

	groups := []SubtitleGroup{{Indices: []int{0}, Texts: []string{items[0].Text}}}

	for i := 1; i < len(items); i++ {
		gap := items[i].StartAt - items[i-1].EndAt
		if gap.Seconds() <= maxGapSeconds {
			groups[len(groups)-1].Indices = append(groups[len(groups)-1].Indices, i)
			groups[len(groups)-1].Texts = append(groups[len(groups)-1].Texts, items[i].Text)
		} else {
			groups = append(groups, SubtitleGroup{
				Indices: []int{i},
				Texts:   []string{items[i].Text},
			})
		}
	}

	return groups
}

func (t *Translator) TranslateGroups(ctx context.Context, groups []SubtitleGroup) (map[int]string, error) {
	results := make(map[int]string)

	for _, group := range groups {
		log.Printf("[分组翻译] 翻译包含 %d 条字幕的分组", len(group.Indices))

		jsonArray, err := json.Marshal(group.Texts)
		if err != nil {
			log.Printf("[分组翻译] JSON序列化失败: %v", err)
			return nil, fmt.Errorf("failed to marshal JSON: %w", err)
		}

		systemPrompt := fmt.Sprintf(translatePrompt, len(group.Texts), len(group.Texts))
		if t.context != "" {
			systemPrompt = fmt.Sprintf(translateWithContextPrompt, len(group.Texts), len(group.Texts), t.context)
		}

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(string(jsonArray)),
		}

		var translations []string
		maxRetries := 3

	outer:
		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				log.Printf("[分组翻译] 第 %d 次重试", retry)
			}

			// 将 expectedCount 存入 context
			ctxWithUserData := context.WithValue(ctx, submitTranslationUserdataKey, &SubmitTranslationUserdata{
				ExpectedCount: len(group.Indices),
			})

			log.Printf("[分组翻译] 原始输入: %s", string(jsonArray))
			resp, err := t.model.Generate(ctxWithUserData, messages)
			if err != nil {
				log.Printf("[分组翻译] 翻译失败: %v", err)
				return nil, fmt.Errorf("failed to translate group: %w", err)
			}

			log.Printf("[分组翻译] 响应内容: %s", string(resp.Content))
			log.Printf("[分组翻译] ToolCalls 数据: %s", tojson(resp.ToolCalls))

			// 处理tool_call，通过tool_call拿到翻译结果
			if len(resp.ToolCalls) > 0 {
				for _, toolCall := range resp.ToolCalls {
					if toolCall.Function.Name == "submit_translation" {
						var input SubmitTranslationReq
						if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
							log.Printf("[分组翻译] ToolCall 参数解析失败: %v", err)
							if retry < maxRetries-1 {
								messages = append(messages, schema.AssistantMessage(string(resp.Content), resp.ToolCalls))
								messages = append(messages, schema.UserMessage("翻译提交失败，请重新提交。"))
								continue outer
							}
							return nil, fmt.Errorf("failed to parse tool call arguments: %w", err)
						}

						log.Printf("[分组翻译] 收到翻译结果，共 %d 条", len(input.Translations))
						translations = input.Translations

						// 调用工具验证
						validateTool := &SubmitTranslationTool{}
						toolResult, err := validateTool.InvokableRun(ctxWithUserData, toolCall.Function.Arguments)
						if err != nil {
							log.Printf("[分组翻译] 工具调用失败: %v", err)
							if retry < maxRetries-1 {
								messages = append(messages, schema.AssistantMessage(string(resp.Content), resp.ToolCalls))
								messages = append(messages, schema.ToolMessage(string(toolResult), toolCall.ID))
								messages = append(messages, schema.UserMessage("工具调用错误，请重新提交翻译。"))
								continue outer
							}
							return nil, fmt.Errorf("tool invocation failed: %w", err)
						}

						var validateOutput SubmitTranslationResp
						if err := json.Unmarshal([]byte(toolResult), &validateOutput); err != nil {
							log.Printf("[分组翻译] 验证结果解析失败: %v", err)
							if retry < maxRetries-1 {
								messages = append(messages, schema.AssistantMessage(string(resp.Content), resp.ToolCalls))
								messages = append(messages, schema.ToolMessage(string(toolResult), toolCall.ID))
								messages = append(messages, schema.UserMessage("验证结果解析失败，请重新提交翻译。"))
								continue outer
							}
							return nil, fmt.Errorf("failed to parse validation result: %w", err)
						}

						if validateOutput.Valid {
							log.Printf("[分组翻译] 验证通过")
							break outer
						} else {
							log.Printf("[分组翻译] 验证失败: %s", validateOutput.Reason)
							if retry < maxRetries-1 {
								messages = append(messages, schema.AssistantMessage(string(resp.Content), resp.ToolCalls))
								messages = append(messages, schema.ToolMessage(validateOutput.Reason, toolCall.ID))
								messages = append(messages, schema.UserMessage(validateOutput.Reason+" 请重新翻译并提交。"))
								continue outer
							}
						}
					}
				}
				break
			} else {
				// 没有使用 tool_call，尝试从响应中解析
				log.Printf("[分组翻译] 未检测到 tool_call，尝试从响应中解析")
				err := json.Unmarshal([]byte(resp.Content), &translations)
				if err != nil {
					log.Printf("[分组翻译] JSON解析失败: %v", err)
					if retry < maxRetries-1 {
						messages = append(messages, schema.AssistantMessage(string(resp.Content), resp.ToolCalls))
						messages = append(messages, schema.UserMessage("请使用 submit_translation 工具提交翻译结果。"))
						continue outer
					}
					return nil, fmt.Errorf("failed to parse translation: %w", err)
				}

				if len(translations) != len(group.Indices) {
					log.Printf("[分组翻译] 警告: 翻译后数组长度 %d 与输入数组长度 %d 不匹配", len(translations), len(group.Indices))
					if retry < maxRetries-1 {
						messages = append(messages, schema.AssistantMessage(string(resp.Content), resp.ToolCalls))
						messages = append(messages, schema.UserMessage(fmt.Sprintf("翻译数组长度不匹配。请使用 submit_translation 工具重新提交，确保数组里每一个输入都对应一个输出。")))
						continue outer
					}
				}
				break outer
			}
		}

		for i, idx := range group.Indices {
			if i < len(translations) {
				results[idx] = translations[i]
			} else {
				results[idx] = group.Texts[i]
			}
		}

		log.Printf("[分组翻译] 分组翻译完成")
	}

	return results, nil
}

func tojson(v interface{}) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
