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
	"testing"
	"time"

	"glm-compatible/internal/adapters/openai/shared"
)

func TestRegisterItemsStoresClone(t *testing.T) {
	store := New()
	item := shared.Map{"id": "item_1", "content": "hello"}
	store.RegisterItems([]shared.Map{item})
	item["content"] = "mutated"

	stored, ok := store.Item("item_1")
	if !ok {
		t.Fatal("item was not stored")
	}
	if stored["content"] != "hello" {
		t.Fatalf("stored item was mutated: %#v", stored)
	}
}

func TestResponseLifecycle(t *testing.T) {
	store := New()
	response := shared.Map{"id": "resp_1", "status": "completed"}
	context := []shared.Map{{"id": "msg_in", "role": "user"}}
	output := []shared.Map{{"id": "msg_out", "role": "assistant"}}
	store.SaveResponse(response, context, output, true, "", nil)

	got, ok := store.Response("resp_1")
	if !ok || got["status"] != "completed" {
		t.Fatalf("response = %#v ok=%v", got, ok)
	}
	input, ok := store.ResponseInput("resp_1")
	if !ok || len(input) != 1 || input[0]["id"] != "msg_in" {
		t.Fatalf("input = %#v ok=%v", input, ok)
	}
	full, ok := store.ResponseContext("resp_1")
	if !ok || len(full) != 2 {
		t.Fatalf("context = %#v ok=%v", full, ok)
	}
	updated, ok := store.UpdateResponse("resp_1", func(item shared.Map) shared.Map {
		item["status"] = "cancelled"
		return item
	})
	if !ok || updated["status"] != "cancelled" {
		t.Fatalf("updated = %#v ok=%v", updated, ok)
	}
	if !store.DeleteResponse("resp_1") || store.DeleteResponse("resp_1") {
		t.Fatalf("delete response returned unexpected result")
	}
	if item, ok := store.Item("msg_in"); ok {
		t.Fatalf("deleted response input item still indexed: %#v", item)
	}
	if item, ok := store.Item("msg_out"); ok {
		t.Fatalf("deleted response output item still indexed: %#v", item)
	}
}

func TestUnstoredResponseDoesNotKeepContext(t *testing.T) {
	store := New()
	store.SaveResponse(shared.Map{"id": "resp_1"}, []shared.Map{{"id": "msg_in"}}, nil, false, "", nil)
	if _, ok := store.Response("resp_1"); ok {
		t.Fatal("response should not be stored when store=false")
	}
	if input, ok := store.ResponseInput("resp_1"); ok || input != nil {
		t.Fatalf("unstored response kept input context: %#v ok=%v", input, ok)
	}
	if item, ok := store.Item("msg_in"); ok {
		t.Fatalf("unstored response input item still indexed: %#v", item)
	}
}

func TestConversationLifecycleAndResponseAppend(t *testing.T) {
	store := New()
	store.SaveConversation(shared.Map{"id": "conv_1", "metadata": shared.Map{"topic": "demo"}}, []shared.Map{{"id": "msg_1"}})
	if conv, ok := store.Conversation("conv_1"); !ok || conv["id"] != "conv_1" {
		t.Fatalf("conversation = %#v ok=%v", conv, ok)
	}
	updated, ok := store.UpdateConversation("conv_1", shared.Map{"topic": "updated"})
	if !ok || updated["metadata"].(map[string]any)["topic"] != "updated" {
		t.Fatalf("updated conversation = %#v ok=%v", updated, ok)
	}
	store.SaveResponse(shared.Map{"id": "resp_1"}, []shared.Map{}, []shared.Map{{"id": "msg_out"}}, true, "conv_1", []shared.Map{{"id": "msg_in"}})
	items, ok := store.ConversationItemsFor("conv_1")
	if !ok || len(items) != 3 {
		t.Fatalf("conversation items = %#v ok=%v", items, ok)
	}
	if !store.DeleteConversation("conv_1") || store.DeleteConversation("conv_1") {
		t.Fatalf("delete conversation returned unexpected result")
	}
	for _, id := range []string{"msg_1", "msg_in"} {
		if item, ok := store.Item(id); ok {
			t.Fatalf("deleted conversation item %s still indexed: %#v", id, item)
		}
	}
	if _, ok := store.Item("msg_out"); !ok {
		t.Fatal("response output item was deleted while response still references it")
	}
	if !store.DeleteResponse("resp_1") {
		t.Fatal("delete response failed")
	}
	if item, ok := store.Item("msg_out"); ok {
		t.Fatalf("deleted response output item still indexed: %#v", item)
	}
}

func TestDeleteKeepsItemsReferencedElsewhere(t *testing.T) {
	store := New()
	sharedItem := shared.Map{"id": "msg_shared"}
	store.SaveConversation(shared.Map{"id": "conv_1"}, []shared.Map{sharedItem})
	store.SaveResponse(shared.Map{"id": "resp_1"}, []shared.Map{sharedItem}, nil, true, "", nil)

	if !store.DeleteResponse("resp_1") {
		t.Fatal("delete response failed")
	}
	if _, ok := store.Item("msg_shared"); !ok {
		t.Fatal("shared item was deleted while conversation still references it")
	}
	if !store.DeleteConversation("conv_1") {
		t.Fatal("delete conversation failed")
	}
	if item, ok := store.Item("msg_shared"); ok {
		t.Fatalf("unreferenced shared item still indexed: %#v", item)
	}
}

func TestItemRefCountHandlesOverwriteAndDuplicateConversationItems(t *testing.T) {
	store := New()
	store.SaveConversation(shared.Map{"id": "conv_1"}, []shared.Map{{"id": "msg_old"}})
	store.SaveConversation(shared.Map{"id": "conv_1"}, []shared.Map{{"id": "msg_new"}, {"id": "msg_new"}})

	if _, ok := store.Item("msg_old"); ok {
		t.Fatal("overwritten conversation item was not released")
	}
	if _, ok := store.Item("msg_new"); !ok {
		t.Fatal("new conversation item was not indexed")
	}
	if !store.DeleteConversation("conv_1") {
		t.Fatal("delete conversation failed")
	}
	if item, ok := store.Item("msg_new"); ok {
		t.Fatalf("duplicate conversation item leaked after delete: %#v", item)
	}
}

func TestItemRefCountHandlesResponseOverwrite(t *testing.T) {
	store := New()
	store.SaveResponse(shared.Map{"id": "resp_1"}, []shared.Map{{"id": "msg_old"}}, nil, true, "", nil)
	store.SaveResponse(shared.Map{"id": "resp_1"}, []shared.Map{{"id": "msg_new"}}, nil, true, "", nil)

	if _, ok := store.Item("msg_old"); ok {
		t.Fatal("overwritten response item was not released")
	}
	if _, ok := store.Item("msg_new"); !ok {
		t.Fatal("new response item was not indexed")
	}
	if !store.DeleteResponse("resp_1") {
		t.Fatal("delete response failed")
	}
	if item, ok := store.Item("msg_new"); ok {
		t.Fatalf("response item leaked after delete: %#v", item)
	}
}

func TestUnstoredResponseStillAppendsConversationItems(t *testing.T) {
	store := New()
	store.SaveConversation(shared.Map{"id": "conv_1"}, nil)
	store.SaveResponse(shared.Map{"id": "resp_1"}, nil, []shared.Map{{"id": "msg_out"}}, false, "conv_1", []shared.Map{{"id": "msg_in"}})

	if _, ok := store.Response("resp_1"); ok {
		t.Fatal("response should not be stored when store=false")
	}
	items, ok := store.ConversationItemsFor("conv_1")
	if !ok || len(items) != 2 {
		t.Fatalf("conversation items = %#v ok=%v", items, ok)
	}
	for _, id := range []string{"msg_in", "msg_out"} {
		if _, ok := store.Item(id); !ok {
			t.Fatalf("conversation item %s was not indexed", id)
		}
	}
}

func TestResponseDoesNotRecreateDeletedConversationItems(t *testing.T) {
	store := New()
	store.SaveConversation(shared.Map{"id": "conv_1"}, []shared.Map{{"id": "msg_1"}})
	if !store.DeleteConversation("conv_1") {
		t.Fatal("delete conversation failed")
	}

	store.SaveResponse(shared.Map{"id": "resp_1"}, nil, []shared.Map{{"id": "msg_out"}}, false, "conv_1", []shared.Map{{"id": "msg_in"}})
	if items, ok := store.ConversationItemsFor("conv_1"); ok || items != nil {
		t.Fatalf("deleted conversation was recreated: %#v ok=%v", items, ok)
	}
	for _, id := range []string{"msg_in", "msg_out"} {
		if item, ok := store.Item(id); ok {
			t.Fatalf("orphan conversation item %s still indexed: %#v", id, item)
		}
	}
}

func TestChatCompletionLifecycleAndFiltering(t *testing.T) {
	store := New()
	store.SaveChatCompletion(
		shared.Map{"id": "chat_2", "created": 2, "model": "glm-5.1", "metadata": shared.Map{"topic": "skip"}},
		[]shared.Map{{"id": "msg_2"}},
	)
	store.SaveChatCompletion(
		shared.Map{"id": "chat_1", "created": 1, "model": "glm-5.1", "metadata": shared.Map{"topic": "demo"}},
		[]shared.Map{{"id": "msg_1"}},
	)
	items := store.ListChatCompletions("glm-5.1", map[string]string{"topic": "demo"})
	if len(items) != 1 || items[0]["id"] != "chat_1" {
		t.Fatalf("filtered items = %#v", items)
	}
	updated, ok := store.UpdateChatCompletion("chat_1", shared.Map{"topic": "updated"})
	if !ok || updated["metadata"].(map[string]any)["topic"] != "updated" {
		t.Fatalf("updated chat = %#v ok=%v", updated, ok)
	}
	messages, ok := store.ChatCompletionMessagesFor("chat_1")
	if !ok || len(messages) != 1 || messages[0]["id"] != "msg_1" {
		t.Fatalf("messages = %#v ok=%v", messages, ok)
	}
	if !store.DeleteChatCompletion("chat_1") || store.DeleteChatCompletion("chat_1") {
		t.Fatalf("delete chat returned unexpected result")
	}
}

func TestStoreEvictsOldResponses(t *testing.T) {
	store := NewWithLimits(Limits{MaxResponses: 1})
	store.SaveResponse(shared.Map{"id": "resp_1"}, nil, []shared.Map{{"id": "msg_1"}}, true, "", nil)
	store.SaveResponse(shared.Map{"id": "resp_2"}, nil, []shared.Map{{"id": "msg_2"}}, true, "", nil)

	if _, ok := store.Response("resp_1"); ok {
		t.Fatal("old response was not evicted")
	}
	if _, ok := store.Item("msg_1"); ok {
		t.Fatal("old response item was not released")
	}
	if _, ok := store.Response("resp_2"); !ok {
		t.Fatal("new response was evicted")
	}
}

func TestStoreEvictsOldConversations(t *testing.T) {
	store := NewWithLimits(Limits{MaxConversations: 1})
	store.SaveConversation(shared.Map{"id": "conv_1"}, []shared.Map{{"id": "msg_1"}})
	store.SaveConversation(shared.Map{"id": "conv_2"}, []shared.Map{{"id": "msg_2"}})

	if _, ok := store.Conversation("conv_1"); ok {
		t.Fatal("old conversation was not evicted")
	}
	if _, ok := store.Item("msg_1"); ok {
		t.Fatal("old conversation item was not released")
	}
	if _, ok := store.Conversation("conv_2"); !ok {
		t.Fatal("new conversation was evicted")
	}
}

func TestStoreEvictionKeepsSharedItems(t *testing.T) {
	store := NewWithLimits(Limits{MaxResponses: 1, MaxConversations: 1})
	sharedItem := shared.Map{"id": "msg_shared"}
	store.SaveConversation(shared.Map{"id": "conv_1"}, []shared.Map{sharedItem})
	store.SaveResponse(shared.Map{"id": "resp_1"}, []shared.Map{sharedItem}, nil, true, "", nil)
	store.SaveResponse(shared.Map{"id": "resp_2"}, nil, []shared.Map{{"id": "msg_2"}}, true, "", nil)

	if _, ok := store.Response("resp_1"); ok {
		t.Fatal("old response was not evicted")
	}
	if _, ok := store.Item("msg_shared"); !ok {
		t.Fatal("shared item was deleted while conversation still references it")
	}

	store.SaveConversation(shared.Map{"id": "conv_2"}, []shared.Map{{"id": "msg_3"}})
	if _, ok := store.Conversation("conv_1"); ok {
		t.Fatal("old conversation was not evicted")
	}
	if _, ok := store.Item("msg_shared"); ok {
		t.Fatal("shared item survived after all references were evicted")
	}
}

func TestStoreEvictsChatCompletions(t *testing.T) {
	store := NewWithLimits(Limits{MaxChatCompletions: 1})
	store.SaveChatCompletion(shared.Map{"id": "chat_1"}, []shared.Map{{"id": "msg_1"}})
	store.SaveChatCompletion(shared.Map{"id": "chat_2"}, []shared.Map{{"id": "msg_2"}})

	if _, ok := store.ChatCompletion("chat_1"); ok {
		t.Fatal("old chat completion was not evicted")
	}
	if messages, ok := store.ChatCompletionMessagesFor("chat_1"); ok || messages != nil {
		t.Fatalf("old chat messages still available: %#v ok=%v", messages, ok)
	}
	if _, ok := store.ChatCompletion("chat_2"); !ok {
		t.Fatal("new chat completion was evicted")
	}
}

func TestStoreLimitZeroMeansUnlimited(t *testing.T) {
	store := NewWithLimits(Limits{})
	for _, id := range []string{"resp_1", "resp_2"} {
		store.SaveResponse(shared.Map{"id": id}, nil, nil, true, "", nil)
	}
	for _, id := range []string{"chat_1", "chat_2"} {
		store.SaveChatCompletion(shared.Map{"id": id}, nil)
	}
	for _, id := range []string{"conv_1", "conv_2"} {
		store.SaveConversation(shared.Map{"id": id}, nil)
	}

	stats := store.Stats()
	if stats.Responses != 2 || stats.ChatCompletions != 2 || stats.Conversations != 2 {
		t.Fatalf("unexpected stats with unlimited store: %#v", stats)
	}
}

func TestStoreTTLEvictsUnaccessedEntries(t *testing.T) {
	store := NewWithLimits(Limits{TTL: time.Second})
	now := time.Unix(100, 0)
	store.now = func() time.Time { return now }
	store.SaveResponse(shared.Map{"id": "resp_1"}, nil, []shared.Map{{"id": "msg_1"}}, true, "", nil)

	now = now.Add(500 * time.Millisecond)
	if _, ok := store.Response("resp_1"); !ok {
		t.Fatal("response expired before ttl")
	}
	now = now.Add(1100 * time.Millisecond)
	store.Prune()

	if _, ok := store.Response("resp_1"); ok {
		t.Fatal("response was not pruned after ttl")
	}
	if _, ok := store.Item("msg_1"); ok {
		t.Fatal("expired response item was not released")
	}
}
