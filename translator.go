package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Translator struct {
	model   model.ToolCallingChatModel
	context string
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

	return &Translator{
		model:   chatModel,
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
		schema.SystemMessage("You are a movie expert. Analyze the provided subtitle samples and filename to summarize the movie/series background, genre, main themes, and context that would help with accurate translation. Keep it concise (2-3 sentences)."),
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

		systemPrompt := fmt.Sprintf("You are a professional subtitle translator. I will give you a JSON array containing %d English subtitle lines. Your task is to translate each line to Chinese and return a JSON array with the same structure. CRITICAL REQUIREMENTS:\n1. Return EXACTLY a JSON array with %d elements\n2. Each input line must become exactly ONE output translation\n3. Do NOT change the JSON structure\n4. Do NOT add or remove any array elements\n5. Output ONLY the JSON array, no markdown formatting, no explanations\n6. The output must be valid JSON\n\nExample input: [\"Hello\", \"World\"]\nExample output: [\"你好\", \"世界\"]", len(group.Texts), len(group.Texts))
		if t.context != "" {
			systemPrompt = fmt.Sprintf("Context: %s\n\nYou are a professional subtitle translator. I will give you a JSON array containing %d English subtitle lines. Your task is to translate each line to Chinese considering this context and return a JSON array with the same structure. CRITICAL REQUIREMENTS:\n1. Return EXACTLY a JSON array with %d elements\n2. Each input line must become exactly ONE output translation\n3. Do NOT change the JSON structure\n4. Do NOT add or remove any array elements\n5. Output ONLY the JSON array, no markdown formatting, no explanations\n6. The output must be valid JSON\n\nExample input: [\"Hello\", \"World\"]\nExample output: [\"你好\", \"世界\"]", t.context, len(group.Texts), len(group.Texts))
		}

		messages := []*schema.Message{
			schema.SystemMessage(systemPrompt),
			schema.UserMessage(string(jsonArray)),
		}

		resp, err := t.model.Generate(ctx, messages)
		if err != nil {
			log.Printf("[分组翻译] 翻译失败: %v", err)
			return nil, fmt.Errorf("failed to translate group: %w", err)
		}

		translatedLines := resp.Content
		log.Printf("[分组翻译] 原始输入: %s", string(jsonArray))
		log.Printf("[分组翻译] 翻译输出: %s", translatedLines)

		var translations []string
		err = json.Unmarshal([]byte(translatedLines), &translations)
		if err != nil {
			log.Printf("[分组翻译] JSON解析失败: %v", err)
			log.Printf("[分组翻译] 尝试清理markdown格式后重试")
			cleaned := strings.TrimSpace(translatedLines)
			cleaned = strings.TrimPrefix(cleaned, "```json")
			cleaned = strings.TrimPrefix(cleaned, "```")
			cleaned = strings.TrimSuffix(cleaned, "```")
			cleaned = strings.TrimSpace(cleaned)
			err = json.Unmarshal([]byte(cleaned), &translations)
			if err != nil {
				log.Printf("[分组翻译] JSON解析仍然失败: %v", err)
				return nil, fmt.Errorf("failed to parse JSON response: %w", err)
			}
		}

		if len(translations) != len(group.Indices) {
			log.Printf("[分组翻译] 警告: 翻译行数 %d 与输入行数 %d 不匹配", len(translations), len(group.Indices))
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
