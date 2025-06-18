async function runStreamingTest() {
    console.log("=== Running Streaming API Test ===");
    
    // Test 1: EventEmitter-style streaming with doubleStep (non-streaming step)
    console.log("\n--- Test 1: EventEmitter-style streaming with doubleStep ---");
    try {
        const stream = doubleStep.startStream(5.0);
        
        let startReceived = false;
        let finalReceived = false;
        let errorReceived = false;
        
        stream.on('start', (event) => {
            console.log("Start event:", event);
            startReceived = true;
        });
        
        stream.on('final', (event) => {
            console.log("Final event:", event);
            finalReceived = true;
        });
        
        stream.on('error', (event) => {
            console.log("Error event:", event);
            errorReceived = true;
        });
        
        // Wait a bit for events to process
        await new Promise(resolve => setTimeout(resolve, 2000));
        
        console.log("Events received - Start:", startReceived, "Final:", finalReceived, "Error:", errorReceived);
        
        if (!startReceived || !finalReceived || errorReceived) {
            throw new Error("Expected start and final events, but got different results");
        }
        
    } catch (err) {
        console.error("Test 1 failed:", err);
        done(err);
        return;
    }
    
    // Test 2: EventEmitter with ChatStep (potentially streaming step)
    console.log("\n--- Test 2: EventEmitter with ChatStep ---");
    try {
        const factory = new ChatStepFactory();
        const chatStep = factory.newStep();
        
        const conv = new Conversation();
        conv.addMessage("system", "You are a helpful AI assistant. Be concise.");
        conv.addMessage("user", "Say 'Hello World' exactly.");
        
        const stream = chatStep.startStream(conv);
        
        let eventCount = 0;
        let hasStart = false;
        let hasFinal = false;
        let finalText = "";
        
        // Promise to wait for completion
        const testComplete = new Promise((resolve, reject) => {
            stream.on('start', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: start`, event);
                hasStart = true;
            });
            
            stream.on('partial', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: partial`, event.delta);
            });
            
            stream.on('final', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: final`, event.text);
                hasFinal = true;
                finalText = event.text;
                resolve();
            });
            
            stream.on('error', (event) => {
                eventCount++;
                console.log(`Event ${eventCount}: error`, event.error);
                reject(new Error("Received error event: " + event.error));
            });
        });
        
        await testComplete;
        
        console.log("EventEmitter test complete - Events:", eventCount, "HasStart:", hasStart, "HasFinal:", hasFinal);
        
        if (!hasStart || !hasFinal) {
            throw new Error("Expected start and final events");
        }
        
    } catch (err) {
        console.error("Test 2 failed:", err);
        done(err);
        return;
    }
    
    // Test 3: Stream cancellation
    console.log("\n--- Test 3: Stream cancellation ---");
    try {
        const stream = doubleStep.startStream(10.0);
        
        let eventCount = 0;
        stream.on('start', () => eventCount++);
        stream.on('final', () => eventCount++);
        stream.on('error', () => eventCount++);
        
        // Cancel immediately
        stream.cancel();
        
        // Wait a bit to see if events still come through
        await new Promise(resolve => setTimeout(resolve, 1000));
        
        console.log("Events after cancellation:", eventCount);
        // Note: We might still receive some events due to timing, but they should be minimal
        
    } catch (err) {
        console.error("Test 3 failed:", err);
        done(err);
        return;
    }
    
    // Test 4: Mixed usage pattern
    console.log("\n--- Test 4: Mixed EventEmitter and cancellation ---");
    try {
        const factory = new ChatStepFactory();
        const chatStep = factory.newStep();
        
        const conv = new Conversation();
        conv.addMessage("user", "Count from 1 to 3, each number on a new line.");
        
        const stream = chatStep.startStream(conv);
        
        let partialCount = 0;
        let cancelled = false;
        
        stream.on('start', () => {
            console.log("Stream started for counting task");
        });
        
        stream.on('partial', (event) => {
            partialCount++;
            console.log(`Partial ${partialCount}:`, event.delta);
            
            // Cancel after receiving some partial events (if any)
            if (partialCount >= 3) {
                console.log("Cancelling stream after 3 partial events");
                stream.cancel();
                cancelled = true;
            }
        });
        
        stream.on('final', (event) => {
            if (!cancelled) {
                console.log("Final result:", event.text);
            }
        });
        
        stream.on('error', (event) => {
            console.log("Error in mixed test:", event.error);
        });
        
        // Wait for processing
        await new Promise(resolve => setTimeout(resolve, 5000));
        
        console.log("Mixed test complete - Partials:", partialCount, "Cancelled:", cancelled);
        
    } catch (err) {
        console.error("Test 4 failed:", err);
        done(err);
        return;
    }
    
    console.log("\n=== All Streaming Tests Completed Successfully ===");
    done(); // Signal completion
}

console.log("Starting Streaming API Test");
runStreamingTest().catch(err => {
    console.error("Streaming test failed:", err);
    done(err);
});
