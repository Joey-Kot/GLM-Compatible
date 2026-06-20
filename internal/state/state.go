// Copyright (C) 2026 Joey Kot <joey.kot.x@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed WITHOUT ANY WARRANTY; without even the
// implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See <https://www.gnu.org/licenses/> for more details.

package state

import (
	"sync"
	"time"

	"glm-compatible/internal/adapters/openai/shared"
)

type Limits struct {
	MaxResponses       int
	MaxChatCompletions int
	MaxConversations   int
	TTL                time.Duration
	PruneInterval      time.Duration
}

type Stats struct {
	Responses       int `json:"responses"`
	ChatCompletions int `json:"chat_completions"`
	Conversations   int `json:"conversations"`
	Items           int `json:"items"`
}

type Store struct {
	mu sync.RWMutex

	limits    Limits
	now       func() time.Time
	lastPrune time.Time

	Responses              map[string]shared.Map
	ResponseInputItems     map[string][]shared.Map
	ResponseContextItems   map[string][]shared.Map
	responseOrder          []string
	responseAccess         map[string]time.Time
	ChatCompletions        map[string]shared.Map
	ChatCompletionMessages map[string][]shared.Map
	chatCompletionOrder    []string
	chatCompletionAccess   map[string]time.Time
	Conversations          map[string]shared.Map
	ConversationItems      map[string][]shared.Map
	conversationOrder      []string
	conversationAccess     map[string]time.Time
	ItemsByID              map[string]shared.Map
}

func New() *Store {
	return NewWithLimits(Limits{})
}

func NewWithLimits(limits Limits) *Store {
	return &Store{
		limits:                 normalizeLimits(limits),
		now:                    time.Now,
		Responses:              map[string]shared.Map{},
		ResponseInputItems:     map[string][]shared.Map{},
		ResponseContextItems:   map[string][]shared.Map{},
		responseAccess:         map[string]time.Time{},
		ChatCompletions:        map[string]shared.Map{},
		ChatCompletionMessages: map[string][]shared.Map{},
		chatCompletionAccess:   map[string]time.Time{},
		Conversations:          map[string]shared.Map{},
		ConversationItems:      map[string][]shared.Map{},
		conversationAccess:     map[string]time.Time{},
		ItemsByID:              map[string]shared.Map{},
	}
}

func normalizeLimits(limits Limits) Limits {
	if limits.MaxResponses < 0 {
		limits.MaxResponses = 0
	}
	if limits.MaxChatCompletions < 0 {
		limits.MaxChatCompletions = 0
	}
	if limits.MaxConversations < 0 {
		limits.MaxConversations = 0
	}
	if limits.TTL < 0 {
		limits.TTL = 0
	}
	if limits.PruneInterval < 0 {
		limits.PruneInterval = 0
	}
	return limits
}

func (s *Store) Prune() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(s.now(), true)
}

func (s *Store) PruneIfDue() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(s.now(), false)
}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Stats{
		Responses:       len(s.Responses),
		ChatCompletions: len(s.ChatCompletions),
		Conversations:   len(s.Conversations),
		Items:           len(s.ItemsByID),
	}
}

func (s *Store) RegisterItems(items []shared.Map) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registerItemsLocked(items)
}

func (s *Store) registerItemsLocked(items []shared.Map) {
	for _, item := range items {
		id := shared.StringValue(item["id"])
		if id != "" {
			s.ItemsByID[id] = shared.CloneMap(item)
		}
	}
}

func (s *Store) Item(id string) (shared.Map, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.ItemsByID[id]
	return shared.CloneMap(item), ok
}

func (s *Store) Response(id string) (shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.Responses[id]
	if ok {
		s.responseAccess[id] = s.now()
	}
	return shared.CloneMap(item), ok
}

func (s *Store) SaveResponse(response shared.Map, contextItems, outputItems []shared.Map, store bool, conversationID string, currentInputItems []shared.Map) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.pruneExpiredLocked(now, false)
	responseID := shared.StringValue(response["id"])
	if store {
		full := append(shared.CloneSlice(contextItems), shared.CloneSlice(outputItems)...)
		s.ResponseContextItems[responseID] = full
		s.ResponseInputItems[responseID] = shared.CloneSlice(contextItems)
		s.registerItemsLocked(contextItems)
		s.registerItemsLocked(outputItems)
		s.Responses[responseID] = shared.CloneMap(response)
		s.responseOrder = rememberID(s.responseOrder, responseID)
		s.responseAccess[responseID] = now
		s.evictResponsesLocked()
	}
	if conversationID != "" {
		if _, ok := s.Conversations[conversationID]; !ok {
			return
		}
		items := s.ConversationItems[conversationID]
		items = append(items, shared.CloneSlice(currentInputItems)...)
		items = append(items, shared.CloneSlice(outputItems)...)
		s.ConversationItems[conversationID] = items
		s.registerItemsLocked(items)
		s.conversationAccess[conversationID] = now
	}
}

func (s *Store) DeleteResponse(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteResponseLocked(id)
}

func (s *Store) ResponseInput(id string) ([]shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.ResponseInputItems[id]
	if ok {
		s.responseAccess[id] = s.now()
	}
	return shared.CloneSlice(items), ok
}

func (s *Store) ResponseContext(id string) ([]shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, ok := s.ResponseContextItems[id]
	if ok {
		s.responseAccess[id] = s.now()
	}
	return shared.CloneSlice(items), ok
}

func (s *Store) UpdateResponse(id string, fn func(shared.Map) shared.Map) (shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.Responses[id]
	if !ok {
		return nil, false
	}
	updated := fn(shared.CloneMap(item))
	s.Responses[id] = shared.CloneMap(updated)
	s.responseAccess[id] = s.now()
	return shared.CloneMap(updated), true
}

func (s *Store) SaveChatCompletion(completion shared.Map, messages []shared.Map) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.pruneExpiredLocked(now, false)
	id := shared.StringValue(completion["id"])
	s.ChatCompletions[id] = shared.CloneMap(completion)
	s.ChatCompletionMessages[id] = shared.CloneSlice(messages)
	s.chatCompletionOrder = rememberID(s.chatCompletionOrder, id)
	s.chatCompletionAccess[id] = now
	s.evictChatCompletionsLocked()
}

func (s *Store) ChatCompletion(id string) (shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.ChatCompletions[id]
	if ok {
		s.chatCompletionAccess[id] = s.now()
	}
	return shared.CloneMap(item), ok
}

func (s *Store) ChatCompletionMessagesFor(id string) ([]shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.ChatCompletions[id]; !ok {
		return nil, false
	}
	s.chatCompletionAccess[id] = s.now()
	return shared.CloneSlice(s.ChatCompletionMessages[id]), true
}

func (s *Store) ListChatCompletions(model string, metadata map[string]string) []shared.Map {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []shared.Map{}
	for _, completion := range s.ChatCompletions {
		if model != "" && shared.StringValue(completion["model"]) != model {
			continue
		}
		if !matchesMetadata(completion, metadata) {
			continue
		}
		out = append(out, shared.CloneMap(completion))
	}
	shared.SortByCreatedThenID(out)
	return out
}

func (s *Store) UpdateChatCompletion(id string, metadata any) (shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	completion, ok := s.ChatCompletions[id]
	if !ok {
		return nil, false
	}
	completion = shared.CloneMap(completion)
	if metadata == nil {
		completion["metadata"] = shared.Map{}
	} else {
		completion["metadata"] = metadata
	}
	s.ChatCompletions[id] = shared.CloneMap(completion)
	s.chatCompletionAccess[id] = s.now()
	return shared.CloneMap(completion), true
}

func (s *Store) DeleteChatCompletion(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteChatCompletionLocked(id)
}

func (s *Store) SaveConversation(conversation shared.Map, items []shared.Map) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	s.pruneExpiredLocked(now, false)
	id := shared.StringValue(conversation["id"])
	s.Conversations[id] = shared.CloneMap(conversation)
	s.ConversationItems[id] = shared.CloneSlice(items)
	s.registerItemsLocked(items)
	s.conversationOrder = rememberID(s.conversationOrder, id)
	s.conversationAccess[id] = now
	s.evictConversationsLocked()
}

func (s *Store) Conversation(id string) (shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.Conversations[id]
	if ok {
		s.conversationAccess[id] = s.now()
	}
	return shared.CloneMap(item), ok
}

func (s *Store) ConversationItemsFor(id string) ([]shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Conversations[id]; !ok {
		return nil, false
	}
	s.conversationAccess[id] = s.now()
	return shared.CloneSlice(s.ConversationItems[id]), true
}

func (s *Store) UpdateConversation(id string, metadata any) (shared.Map, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, ok := s.Conversations[id]
	if !ok {
		return nil, false
	}
	conversation = shared.CloneMap(conversation)
	if metadata == nil {
		conversation["metadata"] = shared.Map{}
	} else {
		conversation["metadata"] = metadata
	}
	s.Conversations[id] = shared.CloneMap(conversation)
	s.conversationAccess[id] = s.now()
	return shared.CloneMap(conversation), true
}

func (s *Store) DeleteConversation(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deleteConversationLocked(id)
}

func (s *Store) deleteResponseLocked(id string) bool {
	if _, ok := s.Responses[id]; !ok {
		return false
	}
	items := append(shared.CloneSlice(s.ResponseInputItems[id]), s.ResponseContextItems[id]...)
	delete(s.Responses, id)
	delete(s.ResponseInputItems, id)
	delete(s.ResponseContextItems, id)
	delete(s.responseAccess, id)
	s.responseOrder = forgetID(s.responseOrder, id)
	s.deleteUnreferencedItemsLocked(items)
	return true
}

func (s *Store) deleteChatCompletionLocked(id string) bool {
	if _, ok := s.ChatCompletions[id]; !ok {
		return false
	}
	delete(s.ChatCompletions, id)
	delete(s.ChatCompletionMessages, id)
	delete(s.chatCompletionAccess, id)
	s.chatCompletionOrder = forgetID(s.chatCompletionOrder, id)
	return true
}

func (s *Store) deleteConversationLocked(id string) bool {
	if _, ok := s.Conversations[id]; !ok {
		return false
	}
	items := shared.CloneSlice(s.ConversationItems[id])
	delete(s.Conversations, id)
	delete(s.ConversationItems, id)
	delete(s.conversationAccess, id)
	s.conversationOrder = forgetID(s.conversationOrder, id)
	s.deleteUnreferencedItemsLocked(items)
	return true
}

func (s *Store) evictResponsesLocked() {
	limit := s.limits.MaxResponses
	if limit == 0 {
		return
	}
	for len(s.Responses) > limit && len(s.responseOrder) > 0 {
		id := s.responseOrder[0]
		if _, ok := s.Responses[id]; !ok {
			s.responseOrder = s.responseOrder[1:]
			continue
		}
		s.deleteResponseLocked(id)
	}
}

func (s *Store) evictChatCompletionsLocked() {
	limit := s.limits.MaxChatCompletions
	if limit == 0 {
		return
	}
	for len(s.ChatCompletions) > limit && len(s.chatCompletionOrder) > 0 {
		id := s.chatCompletionOrder[0]
		if _, ok := s.ChatCompletions[id]; !ok {
			s.chatCompletionOrder = s.chatCompletionOrder[1:]
			continue
		}
		s.deleteChatCompletionLocked(id)
	}
}

func (s *Store) evictConversationsLocked() {
	limit := s.limits.MaxConversations
	if limit == 0 {
		return
	}
	for len(s.Conversations) > limit && len(s.conversationOrder) > 0 {
		id := s.conversationOrder[0]
		if _, ok := s.Conversations[id]; !ok {
			s.conversationOrder = s.conversationOrder[1:]
			continue
		}
		s.deleteConversationLocked(id)
	}
}

func (s *Store) pruneExpiredLocked(now time.Time, force bool) {
	if s.limits.TTL == 0 {
		return
	}
	if !force && !s.lastPrune.IsZero() && s.limits.PruneInterval > 0 && now.Sub(s.lastPrune) < s.limits.PruneInterval {
		return
	}
	s.lastPrune = now
	cutoff := now.Add(-s.limits.TTL)
	for id, accessed := range s.responseAccess {
		if accessed.Before(cutoff) {
			s.deleteResponseLocked(id)
		}
	}
	for id, accessed := range s.chatCompletionAccess {
		if accessed.Before(cutoff) {
			s.deleteChatCompletionLocked(id)
		}
	}
	for id, accessed := range s.conversationAccess {
		if accessed.Before(cutoff) {
			s.deleteConversationLocked(id)
		}
	}
}

func rememberID(order []string, id string) []string {
	if id == "" {
		return order
	}
	order = forgetID(order, id)
	return append(order, id)
}

func forgetID(order []string, id string) []string {
	for i, value := range order {
		if value == id {
			copy(order[i:], order[i+1:])
			return order[:len(order)-1]
		}
	}
	return order
}

func (s *Store) deleteUnreferencedItemsLocked(items []shared.Map) {
	seen := map[string]bool{}
	for _, item := range items {
		id := shared.StringValue(item["id"])
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if s.itemReferencedLocked(id) {
			continue
		}
		delete(s.ItemsByID, id)
	}
}

func (s *Store) itemReferencedLocked(id string) bool {
	for _, items := range s.ResponseInputItems {
		if sliceHasItemID(items, id) {
			return true
		}
	}
	for _, items := range s.ResponseContextItems {
		if sliceHasItemID(items, id) {
			return true
		}
	}
	for _, items := range s.ConversationItems {
		if sliceHasItemID(items, id) {
			return true
		}
	}
	return false
}

func sliceHasItemID(items []shared.Map, id string) bool {
	for _, item := range items {
		if shared.StringValue(item["id"]) == id {
			return true
		}
	}
	return false
}

func matchesMetadata(item shared.Map, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}
	metadata, ok := item["metadata"].(map[string]any)
	if !ok {
		return false
	}
	for key, value := range filters {
		if shared.StringValue(metadata[key]) != value {
			return false
		}
	}
	return true
}
