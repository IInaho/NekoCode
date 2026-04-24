package chat

import (
	"context"
	"primusbot/llm"
	"sync"
)

type ChatManager struct {
	messages     []llm.Message
	systemPrompt string
	llmClient    llm.LLM
	mu           sync.RWMutex
}

func NewChatManager(systemPrompt string) *ChatManager {
	return &ChatManager{
		systemPrompt: systemPrompt,
		messages:     []llm.Message{},
	}
}

func (cm *ChatManager) SetLLM(client llm.LLM) {
	cm.llmClient = client
}

func (cm *ChatManager) SetSystemPrompt(prompt string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.systemPrompt = prompt
}

func (cm *ChatManager) AddUserMessage(content string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.messages = append(cm.messages, llm.Message{
		Role:    "user",
		Content: content,
	})
}

func (cm *ChatManager) AddAssistantMessage(content string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.messages = append(cm.messages, llm.Message{
		Role:    "assistant",
		Content: content,
	})
}

func (cm *ChatManager) GetMessages() []llm.Message {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	messages := make([]llm.Message, 0, len(cm.messages)+1)
	if cm.systemPrompt != "" {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: cm.systemPrompt,
		})
	}
	messages = append(messages, cm.messages...)

	return messages
}

func (cm *ChatManager) Chat(ctx context.Context, userInput string) (string, error) {
	cm.AddUserMessage(userInput)

	messages := cm.GetMessages()

	resp, err := cm.llmClient.Chat(ctx, messages)
	if err != nil {
		cm.RemoveLastUserMessage()
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", nil
	}

	assistantMessage := resp.Choices[0].Message.Content
	cm.AddAssistantMessage(assistantMessage)

	return assistantMessage, nil
}

func (cm *ChatManager) ChatStream(ctx context.Context, userInput string, tokenCallback func(string, string), doneCallback func()) error {
	cm.AddUserMessage(userInput)

	messages := cm.GetMessages()

	tokenChan, errChan := cm.llmClient.ChatStream(ctx, messages)

	if tokenChan == nil {
		err := <-errChan
		cm.RemoveLastUserMessage()
		return err
	}

	fullResponse := ""
	fullReasoning := ""
	streamDone := make(chan bool)

	go func() {
		for token := range tokenChan {
			fullResponse += token.Content
			fullReasoning += token.ReasoningContent
			tokenCallback(token.Content, token.ReasoningContent)
		}

		if fullResponse != "" {
			cm.AddAssistantMessage(fullResponse)
		} else {
			cm.RemoveLastUserMessage()
		}

		doneCallback()
		streamDone <- true
	}()

	select {
	case err := <-errChan:
		cm.RemoveLastUserMessage()
		return err
	case <-streamDone:
		return nil
	}
}

func (cm *ChatManager) RemoveLastUserMessage() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if len(cm.messages) > 0 && cm.messages[len(cm.messages)-1].Role == "user" {
		cm.messages = cm.messages[:len(cm.messages)-1]
	}
}

func (cm *ChatManager) ClearHistory() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.messages = []llm.Message{}
}

func (cm *ChatManager) GetHistory() []llm.Message {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.messages
}

func (cm *ChatManager) MessageCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.messages)
}
