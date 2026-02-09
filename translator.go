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
        You are a movie/series expert. 
		Analyze the provided subtitle samples and filename to summarize the movie/series background, genre, main themes,
		and context that would help with accurate translation. Keep it concise (2-3 sentences)."
        `
	translatePrompt = `
		You are a professional movie/series subtitle translator. I will give you a JSON array containing %d English subtitle lines. 
		Your task is to translate each line to Chinese considering this context.
		
		IMPORTANT: After translating, you MUST call the submit_translation function to submit your translation, 
		rather than directly outputting it. 
		The function will check if the number of translated lines matches the expected count.
		If validation fails, you must correct your translation and try again.
		
		CRITICAL REQUIREMENTS:
		1. Each input line must become exactly ONE output translation
		2. Do NOT add or remove any array elements
		3. Always use submit_translation to check your work
		4. If validation fails, correct the translation and try again
		
		Expected line count: %d
		`
	translateWithContextPrompt = `
		You are a professional movie/series subtitle translator. I will give you a JSON array containing %d English subtitle lines. 
		Your task is to translate each line to Chinese considering this context.
		
		IMPORTANT: After translating, you MUST call the submit_translation function to submit your translation, 
		rather than directly outputting it. 
		The function will check if the number of translated lines matches the expected count.
		If validation fails, you must correct your translation and try again.
		
		CRITICAL REQUIREMENTS:
		1. Each input line must become exactly ONE output translation
		2. Do NOT add or remove any array elements
		3. Always use submit_translation to check your work
		4. If validation fails, correct the translation and try again
		
		Expected line count: %d
		
		Movie/series context: %s
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
		Desc: "提交翻译结果。该函数会检查翻译后的行数是否与输入行数一致。如果不一致，返回错误信息。",
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
			Reason: fmt.Sprintf("翻译行数 %d 与预期行数 %d 不匹配！请重新翻译，确保每一行输入都对应一行输出。", len(input.Translations), expectedCount),
		}
		result, _ := json.Marshal(output)
		log.Printf("[分组翻译] 输出: %s", string(result))
		return string(result), nil
	}

	output := SubmitTranslationResp{
		Valid:  true,
		Reason: "翻译行数正确",
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

		for retry := 0; retry < maxRetries; retry++ {
			if retry > 0 {
				log.Printf("[分组翻译] 第 %d 次重试", retry)
			}

			log.Printf("[分组翻译] 原始输入: %s", string(jsonArray))
			resp, err := t.model.Generate(ctx, messages)
			if err != nil {
				log.Printf("[分组翻译] 翻译失败: %v", err)
				return nil, fmt.Errorf("failed to translate group: %w", err)
			}

			// 处理tool_call，并通过tool_call拿到翻译结果

			if len(translations) != len(group.Indices) {
				log.Printf("[分组翻译] 警告: 翻译行数 %d 与输入行数 %d 不匹配", len(translations), len(group.Indices))
				if retry < maxRetries-1 {
					messages = append(messages, schema.AssistantMessage(string(resp.Content), resp.ToolCalls))
					messages = append(messages, schema.UserMessage(fmt.Sprintf("验证失败：翻译行数 %d 与预期行数 %d 不匹配。请重新翻译，确保每一行输入都对应一行输出。", len(translations), len(group.Indices))))
					continue
				}
			}

			break
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
