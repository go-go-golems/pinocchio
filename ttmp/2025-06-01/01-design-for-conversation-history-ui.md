# LLM Conversation Manager - Complete Specification

## Overview

The LLM Conversation Manager is a terminal-based application for organizing,
browsing, and managing conversations with Large Language Models (like Claude,
GPT, etc.). It provides a fast, keyboard-driven interface for power users who
want to efficiently manage large collections of AI conversations.

## Core Functionality

### 1. Conversation Management
- **Browse conversations** in list or grid view modes
- **View conversation details** including full message history
- **Edit conversation metadata** (title, summary, tags, category)
- **Soft delete conversations** with recovery option
- **Favorite conversations** for quick access

### 2. Organization System
- **Categories**: Hierarchical classification (Development, Learning, Creative, etc.)
- **Tags**: Flexible labeling system with many-to-many relationships
- **Search**: Full-text search across titles, summaries, content, and tags
- **Filtering**: Multi-criteria filtering by category, tags, date, message count, etc.
- **Sorting**: By date, title, message count, category, or relevance

### 3. Bulk Operations
- **Multi-select conversations** with checkbox-style selection
- **Batch tagging** of multiple conversations
- **Batch categorization** of multiple conversations
- **Batch export** in various formats
- **Batch sharing** with configurable permissions
- **Batch deletion** with confirmation

### 4. Sharing & Collaboration (Out of Scope)
- **Generate shareable links** with unique tokens
- **Configure sharing permissions** (public/private, comments allowed)
- **Set expiration dates** for shared links
- **Track view counts** and access analytics
- **Allow comments** on shared conversations (optional)

### 5. Export System
- **Multiple formats**: JSON, CSV, Markdown
- **Flexible options**: Include/exclude metadata, timestamps, formatting
- **Output modes**: Save to file, copy to clipboard, print to stdout
- **Export history**: Track and cache previous exports
- **Progress indication** for large export operations

### 6. Search & Discovery
- **Real-time search** with instant results
- **Advanced filtering** with multiple criteria
- **Saved searches** for common queries
- **Tag suggestions** based on usage patterns
- **Recently viewed** conversations
- **Popular tags** analytics

### 7. Tree Navigation & Branching
- **Thread visualization** showing conversation alternatives
- **Branch switching** between different conversation paths
- **Sibling navigation** to explore alternative responses
- **Tree path display** showing current position in conversation
- **Branch merging** for combining conversation threads
- **Subtree operations** for managing conversation segments

## User Interface Specification

### Main Views

#### 1. Conversation List View
```
💬 LLM Conversations (143)                    🔍 search term    Sort: Date ↓
────────────────────────────────────────────────────────────────────────────────
Filters: All | Development | Learning | Creative    Tags: react python ml writing
────────────────────────────────────────────────────────────────────────────────

⭐ React Component Design                                   2025-05-28  💬12  Dev
   Discussion about creating reusable React components with proper state...
   react frontend components

  Machine Learning Basics                                  2025-05-27  💬8   Learn
   Introduction to machine learning concepts and Python implementation...
   ml python algorithms
```

**Features:**
- Compact list display with essential information
- Star indicators for favorites
- Message count and date prominently displayed
- Category abbreviations (Dev, Learn, Creative, etc.)
- Tag display with overflow handling
- Cursor navigation with j/k keys
- Multi-selection with space key
- Quick actions with single-letter shortcuts

#### 2. Conversation Detail View
```
💬 React Component Design                                              ⭐ Favorite
────────────────────────────────────────────────────────────────────────────────
Created: 2025-05-28 14:32    Nodes: 12    Root: abc123    Current: def456    Size: 24KB
Tags: react frontend components    Category: Development    Shared: No

Summary: Discussion about creating reusable React components with proper state 
management and TypeScript integration for a modern web application.

Thread View: [Main] [Alt 1] [Alt 2]    Navigation: ← Siblings | ↑ Parent | ↓ Children →
────────────────────────────────────────────────────────────────────────────────
14:32  You    > How do I create reusable React components?
14:33  Claude > Here are the key principles for reusable React components...
       ├─ Alternative response 1: Let me explain React component patterns...
       └─ Alternative response 2: For reusable components, consider these approaches...
14:35  You    > Can you show me an example with TypeScript?
14:36  Claude > Certainly! Here's a TypeScript example...
```

**Features:**
- Full conversation metadata display with tree information
- Tree navigation showing current thread and alternatives
- Branch visualization with tree-like indentation
- Thread switching between conversation alternatives
- Scrollable content with keyboard navigation
- Inline editing of title, summary, tags, and category
- Tree-aware navigation (parent/child/sibling nodes)
- Quick action shortcuts at bottom
- Breadcrumb navigation back to list

#### 3. Search & Filter Panel
```
💬 LLM Conversations                         SEARCH & FILTER MODE
────────────────────────────────────────────────────────────────────────────────
🔍 Search: react component design_

📁 Category: All | Development | Learning | Creative | Lifestyle | Business

🏷️  Tags: [×react] [×frontend]  Available: python ml writing cooking database api

📅 Date Range: Last week | Last month | Last 3 months | All time

📊 Sort: Date ↓ | Title | Messages | Category
```

**Features:**
- Real-time search with instant filtering
- Category selection with visual indicators
- Tag selection with add/remove functionality
- Date range presets and custom ranges
- Sort option selection
- Clear all filters option
- Apply/cancel actions

#### 4. Selection & Bulk Actions
```
💬 LLM Conversations (143)                               Selected: 3 conversations
────────────────────────────────────────────────────────────────────────────────

⭐►React Component Design                                  2025-05-28  💬12  Dev
 ►Machine Learning Basics                                 2025-05-27  💬8   Learn
  Creative Writing Help                                   2025-05-26  💬15  Creative
⭐►Database Optimization                                   2025-05-25  💬6   Dev

────────────────────────────────────────────────────────────────────────────────
Actions: [T]ag  [S]hare  [E]xport  [D]elete  [ESC] Cancel selection
```

**Features:**
- Visual selection indicators (►)
- Selection count display
- Bulk action shortcuts
- Select all/clear all functionality
- Selection preservation across filtering

### Navigation Patterns

#### Keyboard Shortcuts
- **j/k**: Navigate up/down in lists
- **h/l**: Navigate left/right in grids or between conversation threads
- **Enter**: Open selected item
- **Space**: Toggle selection
- **/**: Open search
- **t**: Tag selected items
- **s**: Share selected items
- **e**: Export selected items
- **d**: Delete selected items
- **f**: Toggle favorite
- **c**: Change category
- **Esc**: Cancel current operation/go back
- **?**: Show help

**Tree Navigation (in detail view):**
- **p**: Navigate to parent message
- **n**: Navigate to next sibling
- **P**: Navigate to previous sibling
- **1-9**: Switch to numbered alternative thread
- **[/]**: Navigate between child messages
- **Home**: Go to conversation root
- **End**: Go to conversation end (last message in current thread)

#### Mouse Support (Optional)
- Click to select items
- Double-click to open
- Scroll wheel for navigation
- Click on tags/categories to filter

## Data Model Specification

### Core Entities

#### Conversations (ConversationTree)
- **Primary data structure** for storing LLM conversation trees
- **Tree structure**: root_node_id and last_node_id for tree navigation
- **Metadata fields**: title, summary, creation/update timestamps
- **Classification**: category and tags for organization
- **Status flags**: favorite, shared, archived, deleted
- **Technical info**: file size, tree structure references
- **Sharing**: share tokens and permissions

**Tree Operations:**
- **InsertMessages**: Add new message nodes to the tree
- **AttachThread**: Attach a conversation thread to a specific parent node
- **AppendMessages**: Append messages to the end of the current thread
- **PrependThread**: Add messages at the beginning of the tree
- **GetConversationThread**: Retrieve linear path from root to specified node
- **GetLeftMostThread**: Get the primary conversation path (leftmost branch)
- **FindSiblings**: Find alternative responses at the same conversation point
- **FindChildren**: Get all possible continuations from a message node

#### Message Nodes
- **Tree-structured messages** within conversations using parent-child relationships
- **Unique identification**: UUID-based NodeID for each message
- **Role identification**: user, assistant, system, tool
- **Content types**: chat-message, tool-use, tool-result, image
- **Content storage**: JSON-serialized content based on content type
- **Tree navigation**: parent_id links for building conversation trees
- **Metadata**: timestamps, LLM-specific metadata (engine, temperature, usage)
- **Branching support**: multiple children per node for conversation alternatives

**Content Type Structures:**
- **ChatMessageContent**: Role, text, embedded images
- **ToolUseContent**: Tool ID, name, input parameters, type
- **ToolResultContent**: Tool ID, execution result
- **ImageContent**: URL/binary data, name, media type, detail level

#### Categories
- **Hierarchical organization** structure
- **Visual customization**: icons and colors
- **Predefined categories**: Development, Learning, Creative, etc.
- **Custom categories**: user-defined categories
- **Parent-child relationships**: for nested organization

#### Tags
- **Flexible labeling** system
- **Usage tracking**: popularity and frequency
- **Visual customization**: colors for different tag types
- **Auto-suggestions**: based on content and patterns

#### Sharing
- **Public/private** conversation sharing
- **Unique share tokens** for security
- **Expiration dates** for time-limited access
- **Comment permissions** for collaborative feedback
- **Access analytics**: view counts and timestamps

### Relationships
- **Conversations ↔ Categories**: Many-to-one (one category per conversation)
- **Conversations ↔ Tags**: Many-to-many (multiple tags per conversation)
- **Conversations ↔ Message Nodes**: One-to-many (multiple message nodes per conversation tree)
- **Message Nodes ↔ Message Nodes**: Tree structure (parent-child relationships via parent_id)
- **Conversations ↔ Shares**: One-to-many (multiple share configurations)
- **Message Nodes ↔ Image Content**: One-to-many (multiple images per message)
- **Conversation Tree Paths**: Materialized paths for efficient tree queries (ancestor-descendant relationships)

## Technical Architecture

### Bubble Tea Model Structure

#### AppModel (Main Application)
**Responsibility**: Root application state and screen coordination

**State:**
- Current active screen
- Global selection state
- Status messages and loading states
- Window dimensions

**Messages:**
- Screen navigation commands
- Global keyboard shortcuts
- Window resize events
- Cross-component coordination

#### ConversationListModel
**Responsibility**: Browse and manage conversation collections

**State:**
- Conversation list data
- Current cursor position
- Selection state (multi-select)
- View mode (list/grid)
- Filtering and sorting options
- Pagination state

**Messages:**
- Data loading (conversations, categories, tags)
- Navigation (cursor movement, page changes)
- Selection (toggle, select all, clear)
- View changes (mode, sort, filter)
- Actions (open, favorite, delete)

#### ConversationDetailModel
**Responsibility**: View and edit individual conversations with tree navigation

**State:**
- Current conversation tree data
- Current thread path (root to current node)
- Current node position in tree
- Available alternative threads
- Scroll position
- Edit mode state
- Loading status
- Tree navigation state

**Messages:**
- Data loading (conversation tree, message nodes)
- Tree navigation (parent, children, siblings, thread switching)
- Navigation (scroll, node selection)
- Editing (enter/exit edit mode, save changes)
- Tree operations (branch creation, thread merging)
- Actions (share, export, delete)

#### SearchFilterModel
**Responsibility**: Advanced search and filtering

**State:**
- Search criteria (term, category, tags, date range)
- Available options (categories, tags)
- Focus state for form fields
- Dirty state (unsaved changes)

**Messages:**
- Filter changes (search, category, tags, date)
- Focus management
- Apply/clear/cancel actions
- Data loading (categories, tags)

#### QuickActionsModel
**Responsibility**: Bulk operations on selected conversations

**State:**
- Target conversation IDs
- Available actions
- Current selection

**Messages:**
- Action initialization
- Action selection and execution
- Dialog show/hide

#### ExportDialogModel
**Responsibility**: Configure and execute export operations

**State:**
- Export configuration (format, options, output)
- Progress tracking
- Available formats

**Messages:**
- Configuration changes
- Export process (start, progress, complete, error)
- Dialog management

### Inter-Model Communication

#### Global Messages
- **Navigation**: Screen changes, back navigation
- **Selection**: Cross-screen selection synchronization
- **Data Updates**: Conversation changes, tag updates
- **Status**: Success/error messages, loading states

#### Message Flow Examples
1. **Select conversations in list → Show quick actions**
   - ConversationListModel emits `QuickActionsRequested`
   - AppModel shows QuickActionsModel
   - QuickActionsModel receives `ActionsInitialized`

2. **Edit conversation in detail view → Update list**
   - ConversationDetailModel emits `ConversationUpdated`
   - ConversationListModel receives and updates display
   - AppModel shows status message

3. **Apply filters → Refresh list**
   - SearchFilterModel emits `FiltersApplied`
   - ConversationListModel receives and reloads data
   - ConversationListModel emits `ConversationsFiltered`

## Database Schema

### Core Tables

```sql
-- Main conversations table (represents conversation trees)
CREATE TABLE conversations (
    id TEXT PRIMARY KEY,  -- UUID as string
    title TEXT NOT NULL,
    summary TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    root_node_id TEXT NOT NULL,  -- UUID of the root message node
    last_node_id TEXT NOT NULL,  -- UUID of the last inserted message node
    category_id INTEGER,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
    is_shared BOOLEAN NOT NULL DEFAULT FALSE,
    share_token TEXT UNIQUE,
    share_expires_at DATETIME,
    allow_comments BOOLEAN NOT NULL DEFAULT FALSE,
    file_size_bytes INTEGER DEFAULT 0,
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at DATETIME,
    
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE SET NULL,
    FOREIGN KEY (root_node_id) REFERENCES message_nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (last_node_id) REFERENCES message_nodes(id) ON DELETE CASCADE
);

-- Categories for organizing conversations
CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    color TEXT,
    icon TEXT,
    parent_id INTEGER,
    sort_order INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (parent_id) REFERENCES categories(id) ON DELETE CASCADE
);

-- Tags for flexible labeling
CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    color TEXT,
    usage_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Many-to-many relationship between conversations and tags
CREATE TABLE conversation_tags (
    conversation_id TEXT NOT NULL,
    tag_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (conversation_id, tag_id),
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

-- Message nodes in the conversation tree structure
CREATE TABLE message_nodes (
    id TEXT PRIMARY KEY,  -- UUID as string (NodeID)
    conversation_id TEXT NOT NULL,
    parent_id TEXT,  -- NULL for root nodes, UUID for child nodes
    role TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system', 'tool')),
    content_type TEXT NOT NULL CHECK (content_type IN ('chat-message', 'tool-use', 'tool-result', 'image')),
    content_data TEXT NOT NULL,  -- JSON serialized content based on content_type
    time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_update DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    metadata TEXT,  -- JSON serialized metadata
    llm_metadata TEXT,  -- JSON serialized LLM-specific metadata (engine, temperature, usage, etc.)
    
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES message_nodes(id) ON DELETE CASCADE
);

-- Image content storage for message attachments
CREATE TABLE image_content (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_node_id TEXT NOT NULL,
    image_url TEXT,
    image_content BLOB,
    image_name TEXT NOT NULL,
    media_type TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT 'auto' CHECK (detail IN ('low', 'high', 'auto')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (message_node_id) REFERENCES message_nodes(id) ON DELETE CASCADE
);
```

### Supporting Tables

```sql
-- Sharing and collaboration
CREATE TABLE shares (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_id TEXT NOT NULL,
    share_token TEXT NOT NULL UNIQUE,
    created_by TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    allow_comments BOOLEAN NOT NULL DEFAULT FALSE,
    view_count INTEGER NOT NULL DEFAULT 0,
    last_accessed DATETIME,
    
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

-- Export history for tracking and caching
CREATE TABLE exports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    conversation_ids TEXT NOT NULL,  -- JSON array of conversation UUIDs
    format TEXT NOT NULL CHECK (format IN ('pdf', 'json', 'csv', 'markdown', 'html')),
    filename TEXT NOT NULL,
    file_path TEXT,
    file_size INTEGER,
    include_metadata BOOLEAN NOT NULL DEFAULT TRUE,
    include_timestamps BOOLEAN NOT NULL DEFAULT TRUE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME
);

-- User preferences and settings
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Full-text search index for conversations and message content
CREATE VIRTUAL TABLE conversations_fts USING fts5(
    conversation_id UNINDEXED,
    title,
    summary,
    content,
    tags,
    category,
    content=''
);

-- Tree path materialization for efficient tree queries
CREATE TABLE conversation_tree_paths (
    conversation_id TEXT NOT NULL,
    ancestor_id TEXT NOT NULL,
    descendant_id TEXT NOT NULL,
    depth INTEGER NOT NULL,
    
    PRIMARY KEY (conversation_id, ancestor_id, descendant_id),
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE,
    FOREIGN KEY (ancestor_id) REFERENCES message_nodes(id) ON DELETE CASCADE,
    FOREIGN KEY (descendant_id) REFERENCES message_nodes(id) ON DELETE CASCADE
);
```

### Performance Indexes

```sql
-- Primary navigation and filtering
CREATE INDEX idx_conversations_created_at ON conversations(created_at DESC);
CREATE INDEX idx_conversations_updated_at ON conversations(updated_at DESC);
CREATE INDEX idx_conversations_category ON conversations(category_id);
CREATE INDEX idx_conversations_favorite ON conversations(is_favorite);
CREATE INDEX idx_conversations_shared ON conversations(is_shared);
CREATE INDEX idx_conversations_deleted ON conversations(deleted_at);
CREATE INDEX idx_conversations_root_node ON conversations(root_node_id);
CREATE INDEX idx_conversations_last_node ON conversations(last_node_id);

-- Message node retrieval and tree navigation
CREATE INDEX idx_message_nodes_conversation ON message_nodes(conversation_id);
CREATE INDEX idx_message_nodes_parent ON message_nodes(parent_id);
CREATE INDEX idx_message_nodes_time ON message_nodes(conversation_id, time);
CREATE INDEX idx_message_nodes_content_type ON message_nodes(content_type);
CREATE INDEX idx_message_nodes_role ON message_nodes(role);

-- Tree path queries for efficient ancestor/descendant lookups
CREATE INDEX idx_tree_paths_conversation ON conversation_tree_paths(conversation_id);
CREATE INDEX idx_tree_paths_ancestor ON conversation_tree_paths(ancestor_id);
CREATE INDEX idx_tree_paths_descendant ON conversation_tree_paths(descendant_id);
CREATE INDEX idx_tree_paths_depth ON conversation_tree_paths(conversation_id, depth);

-- Tag relationships
CREATE INDEX idx_conversation_tags_conv ON conversation_tags(conversation_id);
CREATE INDEX idx_conversation_tags_tag ON conversation_tags(tag_id);

-- Image content
CREATE INDEX idx_image_content_message ON image_content(message_node_id);

-- Sharing features
CREATE INDEX idx_shares_token ON shares(share_token);
CREATE INDEX idx_shares_public ON shares(is_public);
```
