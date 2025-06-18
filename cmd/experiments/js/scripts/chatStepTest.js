async function runChatStepTest() {
    console.log("=== Running Chat Step Test ===");
    
    // Create factory and step
    const factory = new ChatStepFactory();
    const chatStep = factory.newStep();
    
    // Create conversation
    const conv = new Conversation();
    conv.AddMessage("system", "You are a helpful AI assistant. Be concise.");
    conv.AddMessage("user", "What is the capital of France?");
    
    // Test Promise API
    console.log("Testing Promise API...");
    try {
        const response = await chatStep.startAsync(conv);
        console.log("Promise response:", response);
        
        // Add assistant's response to conversation
        conv.AddMessage("assistant", response);
    } catch (err) {
        console.error("Promise API error:", err);
        done(err); // Signal error
        return;
    }
    
    // Test Streaming API
    console.log("\nTesting Streaming API...");
    conv.AddMessage("user", "And what is France's population?");
    
    let streamingResponse = "";
    const cancel = chatStep.startWithCallbacks(conv, {
        onResult: (chunk) => {
            streamingResponse += chunk;
            console.log("Chunk received:", chunk);
        },
        onError: (err) => {
            console.error("Streaming error:", err);
            done(err); // Signal error
        },
        onDone: () => {
            console.log("\nFinal streaming response:", streamingResponse);
            
            conv.AddMessage("assistant", streamingResponse);
            console.log("Chat step test complete");
            done(); // Signal completion
        }
    });
    console.log("Streaming started");
}

console.log("Starting ChatStep Test");
runChatStepTest().catch(err => {
    console.error("Test failed:", err);
    done(err); // Signal error
}); 