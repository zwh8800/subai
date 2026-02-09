package main

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudwego/eino/compose"
)

type SubtitleAgent struct {
	chain compose.Runnable[AgentInput, AgentOutput]
}

type AgentInput struct {
	SubtitlePath string
	OutputPath   string
	OutputFormat string
}

type AgentOutput struct {
	Success  bool
	Message  string
	Subtitle *Subtitle
}

func NewSubtitleAgent(ctx context.Context, apiKey string, baseURL string, modelName string) (*SubtitleAgent, error) {
	log.Printf("[Agent] 初始化字幕翻译 Agent，模型: %s", modelName)

	chain := compose.NewChain[AgentInput, AgentOutput]()

	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, input AgentInput) (*Subtitle, error) {
		log.Printf("[Agent] 步骤1: 解析字幕文件: %s", input.SubtitlePath)
		sub, err := ParseSubtitle(input.SubtitlePath)
		if err != nil {
			log.Printf("[Agent] 解析字幕失败: %v", err)
			return nil, err
		}
		log.Printf("[Agent] 解析完成，共 %d 条字幕", len(sub.Items))
		return sub, nil
	}))

	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, sub *Subtitle) (*Subtitle, error) {
		log.Printf("[Agent] 步骤2: 开始翻译 %d 条字幕", len(sub.Items))

		translator, err := NewTranslator(ctx, apiKey, baseURL, modelName)
		if err != nil {
			log.Printf("[Agent] 创建翻译器失败: %v", err)
			return nil, err
		}

		err = translator.SummarizeContext(ctx, inputFile, sub.Items)
		if err != nil {
			log.Printf("[Agent] 总结背景信息失败: %v", err)
		}

		groups := GroupSubtitlesByTime(sub.Items, 3.0)
		log.Printf("[Agent] 将字幕分为 %d 个组进行翻译", len(groups))

		translatedMap, err := translator.TranslateGroups(ctx, groups)
		if err != nil {
			log.Printf("[Agent] 分组翻译失败: %v", err)
			return nil, err
		}

		for i, item := range sub.Items {
			if trans, ok := translatedMap[i]; ok {
				item.Chinese = trans
			}
		}

		log.Printf("[Agent] 翻译完成")
		return sub, nil
	}))

	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, sub *Subtitle) (AgentOutput, error) {
		log.Printf("[Agent] 步骤3: 准备生成输出")
		return AgentOutput{
			Success:  true,
			Message:  "subtitle translated successfully",
			Subtitle: sub,
		}, nil
	}))

	compiledChain, err := chain.Compile(ctx)
	if err != nil {
		log.Printf("[Agent] 编译 Chain 失败: %v", err)
		return nil, fmt.Errorf("failed to compile chain: %w", err)
	}

	log.Printf("[Agent] Agent 初始化完成")
	return &SubtitleAgent{
		chain: compiledChain,
	}, nil
}

func (a *SubtitleAgent) Run(ctx context.Context, input AgentInput) (AgentOutput, error) {
	log.Printf("[Agent] 开始运行 Agent，输入: %+v", input)

	output, err := a.chain.Invoke(ctx, input)
	if err != nil {
		log.Printf("[Agent] 运行失败: %v", err)
		return output, err
	}

	if output.Success && output.Subtitle != nil {
		log.Printf("[Agent] 步骤4: 生成 %s 格式输出", input.OutputFormat)
		var content string
		switch input.OutputFormat {
		case "srt", "SRT":
			content = output.Subtitle.GenerateSRT()
		case "ass", "ASS":
			content = output.Subtitle.GenerateASS()
		default:
			content = output.Subtitle.GenerateSRT()
		}

		log.Printf("[Agent] 保存输出到: %s", input.OutputPath)
		err := saveToFile(input.OutputPath, content)
		if err != nil {
			log.Printf("[Agent] 保存输出失败: %v", err)
			return AgentOutput{
				Success: false,
				Message: fmt.Sprintf("failed to save output: %v", err),
			}, err
		}

		output.Message = fmt.Sprintf("subtitle translated successfully, saved to %s", input.OutputPath)
		log.Printf("[Agent] 运行成功: %s", output.Message)
	}

	return output, nil
}
