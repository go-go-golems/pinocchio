# Refactoring Plan for Conversation Tree API

## 1. Introduction

This document outlines a refactoring plan for the `conversation` package API. The goal is to enhance clarity, maintainability, and separation of concerns by adopting a hybrid naming approach and structuring the API into distinct layers for storage, business logic, and navigation/querying. This plan is derived from the "Hybrid Approach" and structural improvement suggestions in `02-suggestions-for-refactoring-renaming-conversation-tree--sonnet.md`.

## 2. Core Principles

The refactoring will adhere to the following core principles:

-   **Hybrid Naming Convention**: Adopt names that balance domain relevance with technical clarity.
-   **Separation of Concerns**: Clearly delineate responsibilities into:
    -   Data Storage (`ConversationStore`)
    -   Business Logic (`ConversationManager`)
    -   Data Querying & Navigation (`ConversationNavigator`)
-   **Improved Method Clarity**: Use intuitive and precise names for API methods.
-   **Enhanced Testability**: Smaller, focused interfaces will be easier to test.
-   **Maintainability & Extensibility**: A well-defined structure will simplify future modifications and additions.

## 3. Proposed Naming Conventions

The following naming changes are proposed:

| Current Term         | Proposed Term         | Rationale                                   |
| -------------------- | --------------------- | ------------------------------------------- |
| `ConversationTree`   | `ConversationHistory` | Represents the persisted history/structure. |
| `Manager`            | `ConversationManager` | Retains familiarity, role clarified.        |
| `ManagerImpl`        | `ConversationManagerImpl`| Concrete implementation of the manager.     |
| `NodeID`             | `MessageID`           | More specific to messages.                  |
| `Message`            | `ChatMessage`         | Distinguishes from other potential messages.|
| `MessageContent`     | `MessageBody`         | Represents the payload of a `ChatMessage`.  |
| `Conversation` (slice) | `MessageThread`       | A linear sequence of chat messages.         |

## 4. Proposed API Structure

### 4.1. Core Data Structures

```go
// MessageID defines the unique identifier for a message.
type MessageID string // Typically a UUID

// ConversationHistory holds the entire tree structure of a conversation.
// It replaces the old ConversationTree.
type ConversationHistory struct {
    ConversationID string // Unique ID for the entire conversation history
    Title          string
    RootID         MessageID
    // CurrentActiveMessageID might be useful for tracking the tip of the "main" or currently focused branch.
    CurrentActiveMessageID MessageID
    Messages      map[MessageID]*ChatMessage
    CreatedAt     time.Time
    UpdatedAt     time.Time
    Metadata      map[string]interface{} // For conversation-level metadata
}

// ChatMessage represents a single node in the conversation tree.
// It replaces the old Message.
type ChatMessage struct {
    ID         MessageID
    ParentID   MessageID // Nil or empty for the root message
    Body       MessageBody
    Timestamp  time.Time
    LastUpdate time.Time
    Metadata   map[string]interface{} // For message-level metadata (e.g., LLM specifics, ratings)
    
    // Children are implicitly defined by other messages referencing this message's ID as ParentID.
    // For easier navigation, a navigator can resolve this.
}

// MessageBody is an interface for the content of a ChatMessage.
// It replaces the old MessageContent.
type MessageBody interface {
    ContentType() string // e.g., "text", "tool-use", "tool-result", "image"
    Render() string      // A string representation for display or processing
    Raw() interface{}    // Access to the underlying raw data
}

// Example concrete MessageBody types:
type TextMessageBody struct {
    Role Role // user, assistant, system, tool
    Text string
}
// ... other types like ToolUseBody, ToolResultBody, ImageBody

// Role defines the originator of a message.
type Role string
const (
    RoleSystem    Role = "system"
    RoleAssistant Role = "assistant"
    RoleUser      Role = "user"
    RoleTool      Role = "tool" // For the output/result of a tool, the tool use itself is part of Assistant's MessageBody
)

// MessageThread represents a linear sequence of messages, typically a path in the ConversationHistory.
type MessageThread []*ChatMessage
```

### 4.2. Separated Concerns: Interfaces

#### 4.2.1. `ConversationStore` (Storage Layer)

Responsible for persistence of `ConversationHistory`.

```go
type ConversationStore interface {
    // Save stores or updates an entire conversation history.
    SaveHistory(ctx context.Context, history *ConversationHistory) error
    
    // LoadHistory retrieves a conversation history by its ID.
    LoadHistory(ctx context.Context, conversationID string) (*ConversationHistory, error)
    
    // DeleteHistory removes a conversation history.
    DeleteHistory(ctx context.Context, conversationID string) error
    
    // ListHistories provides a way to get multiple conversation summaries/IDs (pagination might be needed).
    ListHistories(ctx context.Context, offset, limit int) ([]*ConversationHistorySummary, error)

    // AddMessageNode adds or updates a single message node within a history.
    // This could be useful for streaming or partial updates, though managing consistency is key.
    // Alternatively, all modifications go through ConversationManager which then calls SaveHistory.
    // For simplicity, we might initially require full history saves.
    // AddMessageNode(ctx context.Context, conversationID string, message *ChatMessage) error 
}

type ConversationHistorySummary struct {
    ConversationID string
    Title          string
    LastUpdatedAt  time.Time
    MessageCount   int // Approximate or derived
}
```

#### 4.2.2. `ConversationManager` (Business Logic Layer)

Responsible for orchestrating conversation flows and modifications to a `ConversationHistory` object (which might be in-memory or loaded/persisted via a `ConversationStore`).

```go
// ConversationManager manages the lifecycle and state of conversations.
// Implementations (e.g., ConversationManagerImpl) would use a ConversationStore for persistence.
type ConversationManager interface {
    // NewConversation creates a new ConversationHistory instance.
    NewConversation(ctx context.Context, title string, initialMessages ...*ChatMessage) (*ConversationHistory, error)

    // SetTitle updates the title of a conversation.
    SetTitle(ctx context.Context, history *ConversationHistory, title string) error

    // AddMessage appends a new message to a specified parent in the conversation history.
    // If parentID is empty or refers to CurrentActiveMessageID, it extends the active thread.
    // Returns the newly added ChatMessage and potentially the updated ConversationHistory.
    AddMessage(ctx context.Context, history *ConversationHistory, parentID MessageID, body MessageBody, role Role) (*ChatMessage, error)

    // CreateBranch starts a new thread of conversation from a given parent message.
    // This essentially means adding a new message that has parentID as its parent.
    // The 'active' context might switch to this new branch.
    CreateBranch(ctx context.Context, history *ConversationHistory, parentID MessageID, initialBody MessageBody, role Role) (*ChatMessage, error)
    
    // SetActiveMessage sets the current "tip" of the conversation, affecting GetActiveThread.
    SetActiveMessage(ctx context.Context, history *ConversationHistory, messageID MessageID) error
    
    // GetConversationForLLM prepares a MessageThread suitable for sending to an LLM,
    // typically the active thread up to a certain context window.
    GetConversationForLLM(ctx context.Context, history *ConversationHistory, tipMessageID MessageID, maxMessages int, maxLength int) (MessageThread, error)
    
    // DeleteMessage (and its children - soft or hard delete)
    DeleteMessage(ctx context.Context, history *ConversationHistory, messageID MessageID, recursive bool) error
}

// ConversationManagerImpl would be the concrete implementation.
// type ConversationManagerImpl struct {
//     store ConversationStore
// }
```

#### 4.2.3. `ConversationNavigator` (Navigation/Query Layer)

Responsible for querying and navigating the `ConversationHistory` structure. This could be part of the `ConversationManager` or a separate utility.

```go
type ConversationNavigator interface {
    // GetMessage retrieves a specific message by its ID from the history.
    GetMessage(history *ConversationHistory, id MessageID) (*ChatMessage, bool)

    // GetChildren retrieves all direct children of a given message.
    GetChildren(history *ConversationHistory, id MessageID) ([]*ChatMessage, error)

    // GetParent retrieves the parent of a given message.
    GetParent(history *ConversationHistory, id MessageID) (*ChatMessage, error)

    // GetPathToRoot traces a message back to the root, returning the thread.
    GetPathToRoot(history *ConversationHistory, id MessageID) (MessageThread, error)

    // GetMainThread retrieves the primary or "left-most" thread from the root.
    // This definition needs to be precise (e.g., always first child or a marked main branch).
    GetMainThread(history *ConversationHistory) (MessageThread, error)

    // GetActiveThread retrieves the thread leading up to the CurrentActiveMessageID.
    GetActiveThread(history *ConversationHistory) (MessageThread, error)

    // GetSiblings retrieves messages that share the same parent.
    GetSiblings(history *ConversationHistory, id MessageID) ([]*ChatMessage, error)

    // GetRootMessage retrieves the root message of the conversation.
    GetRootMessage(history *ConversationHistory) (*ChatMessage, error)
}
```

## 5. Refined Method Names

Based on the new structure, method names become more contextual:

| Old Method (Context)             | Proposed Method (Interface)        | Notes                                                       |
| -------------------------------- | ---------------------------------- | ----------------------------------------------------------- |
| `Manager.AppendMessages`         | `Manager.AddMessage` (to active)   | `parentID` can target the current active tip.               |
| `Manager.AttachMessages`         | `Manager.AddMessage` (to specific parent) | `parentID` explicitly targets a branch point.             |
| `Manager.GetConversation`        | `Navigator.GetActiveThread`        | Or `Navigator.GetMainThread`.                               |
| `ConversationTree.InsertMessages`| `History.Messages[id] = msg` (internal) | Managed by `ConversationManager.AddMessage`.                |
| `ConversationTree.GetLeftMostThread` | `Navigator.GetMainThread`        | Strategy for "main" needs to be defined.                    |
| `Manager.SaveToFile`             | `Store.SaveHistory`                | `ConversationManager` would use this.                       |
| `Manager.LoadFromFile`           | `Store.LoadHistory`                | `ConversationManager` would use this.                       |

## 6. Implementation Considerations

-   **`ConversationManagerImpl`**: This struct will implement `ConversationManager` and will likely use an instance of `ConversationStore` for persistence and a `ConversationNavigator` (or embed its logic) for querying.
-   **Message ID Generation**: `MessageID`s (UUIDs) should be generated upon message creation.
-   **Concurrency**: Context should be passed through API methods for cancellation and request-scoped values. Appropriate locking mechanisms might be needed if `ConversationHistory` objects are manipulated concurrently, though often they might be loaded, modified, and saved in a transactional manner.
-   **Error Handling**: Consistent error wrapping and handling across layers.
-   **`MessageBody` Serialization**: The `ConversationStore` will need a strategy for serializing/deserializing the `MessageBody` interface (e.g., using a `type` field and `json.RawMessage` or a dedicated serialization mechanism).
-   **Initial Migration**: A script or process will be needed to migrate existing conversation data to the new `ConversationHistory` schema.

## 7. Benefits of this Refactoring

-   **Improved Clarity**: Names and structures are more aligned with standard software design patterns and domain concepts.
-   **Better Separation of Concerns**: Storage, business logic, and navigation are distinct, simplifying development and testing.
-   **Enhanced Testability**: Each component (`Store`, `Manager`, `Navigator`) can be tested in isolation using mocks or fakes.
-   **Easier Extensibility**: Adding new storage backends, modifying business rules, or introducing new navigation features becomes more straightforward.
-   **Scalability**: Clear interfaces allow for different implementations optimized for various scales (e.g., in-memory store vs. distributed database store).

This refactoring provides a solid foundation for a more robust and maintainable conversation management system. 