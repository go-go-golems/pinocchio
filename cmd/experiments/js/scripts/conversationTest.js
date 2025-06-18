async function runConversationTest() {
    console.log("=== Running Conversation Test ===");
    
    const conv = new Conversation();
    console.log("Conversation created:", conv);
    
    try {
        // Test adding messages
        const msgId1 = await conv.AddMessage("system", "You are a helpful assistant.");
        console.log("Added system message:", msgId1);
        
        const msgId2 = await conv.AddMessage("user", "Hello, can you help me?");
        console.log("Added user message:", msgId2);
        
        const msgId3 = await conv.AddMessage("assistant", "Of course! What can I help you with?");
        console.log("Added assistant message:", msgId3);
        
        // Test tool use
        const toolId = "search-123";
        const toolUseId = await conv.AddToolUse(toolId, "search", { query: "test query" });
        console.log("Added tool use:", toolUseId);
        
        const toolResultId = await conv.AddToolResult(toolId, "Found results for test query");
        console.log("Added tool result:", toolResultId);
        
        // Test getting messages
        const messages = conv.GetMessages();
        console.log("Messages count:", messages.length);
        console.log("First message role:", messages[0].Content.Role);
        
        // Test getting single prompt
        const prompt = conv.GetSinglePrompt();
        console.log("Single prompt:", prompt);
        
        // Test message view
        const view = await conv.GetMessageView(msgId1);
        console.log("Message view:", view);
        
        // Test metadata update
        const updated = await conv.UpdateMetadata(msgId1, { processed: true });
        console.log("Metadata updated:", updated);
        
        console.log("Conversation test complete");
    } catch (err) {
        console.error("Test error:", err);
    }
}

runConversationTest().catch(console.error); 