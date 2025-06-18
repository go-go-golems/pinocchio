async function runChatStepStreamingTest() {
    console.log("=== Running Chat Step Streaming Test ===");
    
    // Create factory and step
    const factory = new ChatStepFactory();
    const chatStep = factory.newStep();
    
    // Test 1: Basic streaming with start and final events
    console.log("\n--- Test 1: Basic Streaming Events ---");
    try {
        const conv = new Conversation();
        conv.addMessage("system", "You are a helpful AI assistant. Be concise but informative.");
        conv.addMessage("user", "Explain what JavaScript is in 2-3 sentences.");
        
        const stream = chatStep.startStream(conv);
        
        let eventCount = 0;
        let hasStart = false;
        let hasFinal = false;
        let finalText = "";
        let partialCount = 0;
        
        const testComplete = new Promise((resolve, reject) => {
            stream.on('start', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: START`, event);
                hasStart = true;
            });
            
            stream.on('partial', (event) => {
                eventCount++;
                partialCount++;
                console.log(`Event ${eventCount}: PARTIAL delta="${event.delta}"`);
            });
            
            stream.on('final', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: FINAL text="${event.text}"`);
                hasFinal = true;
                finalText = event.text;
                resolve();
            });
            
            stream.on('error', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: ERROR`, event.error);
                reject(new Error("Received error event: " + event.error));
            });
            
            // Timeout after 30 seconds
            setTimeout(() => {
                reject(new Error("Test timed out after 30 seconds"));
            }, 30000);
        });
        
        await testComplete;
        
        console.log(`Test 1 Results: Events=${eventCount}, Start=${hasStart}, Partials=${partialCount}, Final=${hasFinal}`);
        console.log(`Final response: "${finalText}"`);
        
        if (!hasStart || !hasFinal) {
            throw new Error("Expected start and final events");
        }
        
    } catch (err) {
        console.error("Test 1 failed:", err);
        done(err);
        return;
    }
    
    // Test 2: Longer response to test more partial events
    console.log("\n--- Test 2: Longer Response for More Partials ---");
    try {
        const conv = new Conversation();
        conv.addMessage("system", "You are a knowledgeable assistant. Be detailed in your explanations.");
        conv.addMessage("user", "Write a step-by-step guide on how to make coffee using a French press. Include timing and tips.");
        
        const stream = chatStep.startStream(conv);
        
        let eventCount = 0;
        let partialCount = 0;
        let finalText = "";
        let allPartials = "";
        
        const testComplete = new Promise((resolve, reject) => {
            stream.on('start', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: START`);
            });
            
            stream.on('partial', (event) => {
                eventCount++;
                partialCount++;
                allPartials += event.delta;
                console.log(`Event ${eventCount}: PARTIAL [${event.delta.length} chars] "${event.delta.substring(0, 50)}${event.delta.length > 50 ? '...' : ''}"`);
            });
            
            stream.on('final', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: FINAL [${event.text ? event.text.length : 0} chars total]`);
                finalText = event.text;
                resolve();
            });
            
            stream.on('error', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: ERROR`, event.error);
                reject(new Error("Received error event: " + event.error));
            });
            
            // Longer timeout for detailed response
            setTimeout(() => {
                reject(new Error("Test timed out after 45 seconds"));
            }, 45000);
        });
        
        await testComplete;
        
        console.log(`Test 2 Results: Events=${eventCount}, Partials=${partialCount}`);
        console.log(`All partials combined: [${allPartials.length} chars]`);
        console.log(`Final text: [${finalText ? finalText.length : 0} chars]`);
        
        if (partialCount === 0) {
            console.log("Note: No partial events received - this may be normal for some LLM providers");
        }
        
    } catch (err) {
        console.error("Test 2 failed:", err);
        done(err);
        return;
    }
    
    // Test 3: Stream cancellation
    console.log("\n--- Test 3: Stream Cancellation ---");
    try {
        const conv = new Conversation();
        conv.addMessage("user", "Write a very long story about a dragon and a knight. Make it at least 1000 words with lots of detail.");
        
        const stream = chatStep.startStream(conv);
        
        let eventCount = 0;
        let cancelled = false;
        
        stream.on('start', (event) => {
            eventCount++;
            console.log(`Event ${eventCount}: START - will cancel after 2 seconds`);
            
            // Cancel after 2 seconds
            setTimeout(() => {
                console.log("Cancelling stream...");
                stream.cancel();
                cancelled = true;
            }, 2000);
        });
        
        stream.on('partial', (event) => {
            eventCount++;
            console.log(`Event ${eventCount}: PARTIAL (after cancel=${cancelled}) "${event.delta.substring(0, 30)}..."`);
        });
        
        stream.on('final', (event) => {
            eventCount++;
            console.log(`Event ${eventCount}: FINAL (after cancel=${cancelled})`);
        });
        
        stream.on('error', (event) => {
            eventCount++;
            console.log(`Event ${eventCount}: ERROR`, event.error);
        });
        
        // Wait a bit to see cancellation effects
        await new Promise(resolve => setTimeout(resolve, 5000));
        
        console.log(`Test 3 Results: Events=${eventCount}, Cancelled=${cancelled}`);
        
    } catch (err) {
        console.error("Test 3 failed:", err);
        done(err);
        return;
    }
    
    console.log("\n=== All Chat Step Streaming Tests Completed Successfully ===");
    done(); // Signal completion
}

console.log("Starting Chat Step Streaming Test");
runChatStepStreamingTest().catch(err => {
    console.error("Chat streaming test failed:", err);
    done(err);
});
