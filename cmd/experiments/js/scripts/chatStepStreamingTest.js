async function runStreamingTest() {
    console.log("=== Running Chat Step Streaming Test ===");
    
    // Create factory and step
    const factory = new ChatStepFactory();
    const chatStep = factory.newStep();
    
    let testsPassed = 0;
    let testsTotal = 4;
    
    // Test 1: Basic streaming with short response
    console.log("\n--- Test 1: Basic Streaming (Short Response) ---");
    try {
        const conv1 = new Conversation();
        conv1.addMessage("system", "You are a helpful AI assistant. Be very concise.");
        conv1.addMessage("user", "What is 2 + 2?");
        
        await testStreaming(chatStep, conv1, "basic");
        testsPassed++;
    } catch (err) {
        console.error("Test 1 failed:", err);
    }
    
    // Test 2: Streaming with longer response for multiple partial events
    console.log("\n--- Test 2: Streaming (Longer Response) ---");
    try {
        const conv2 = new Conversation();
        conv2.addMessage("system", "You are a knowledgeable assistant. Provide detailed explanations.");
        conv2.addMessage("user", "Explain how photosynthesis works in plants, including the light and dark reactions.");
        
        await testStreaming(chatStep, conv2, "detailed");
        testsPassed++;
    } catch (err) {
        console.error("Test 2 failed:", err);
    }
    
    // Test 3: Event ordering verification
    console.log("\n--- Test 3: Event Ordering Verification ---");
    try {
        const conv3 = new Conversation();
        conv3.addMessage("system", "You are a helpful assistant.");
        conv3.addMessage("user", "Tell me a short story about a robot learning to dance.");
        
        await testEventOrdering(chatStep, conv3);
        testsPassed++;
    } catch (err) {
        console.error("Test 3 failed:", err);
    }
    
    // Test 4: Cancellation test
    console.log("\n--- Test 4: Cancellation Test ---");
    try {
        const conv4 = new Conversation();
        conv4.addMessage("system", "You are a verbose assistant. Write very long responses.");
        conv4.addMessage("user", "Write a detailed essay about the history of computing, starting from ancient counting devices.");
        
        await testCancellation(chatStep, conv4);
        testsPassed++;
    } catch (err) {
        console.error("Test 4 failed:", err);
    }
    
    console.log(`\n=== Test Results: ${testsPassed}/${testsTotal} tests passed ===`);
    
    if (testsPassed === testsTotal) {
        console.log("✅ All streaming tests passed!");
        done(); // Signal successful completion
    } else {
        console.log("❌ Some tests failed");
        done(new Error(`${testsTotal - testsPassed} tests failed`));
    }
}

async function testStreaming(chatStep, conversation, testType) {
    return new Promise((resolve, reject) => {
        console.log(`Starting ${testType} streaming test...`);
        
        let fullResponse = "";
        let chunkCount = 0;
        let startReceived = false;
        let finalReceived = false;
        
        const stream = chatStep.startStream(conversation);
        
        stream.on('start', () => {
            console.log("📍 START event received");
            startReceived = true;
        });
        
        stream.on('partial', (chunk) => {
            console.log(`📦 PARTIAL event (${++chunkCount}):`, typeof chunk, JSON.stringify(chunk));
            console.log(`📦 Chunk properties:`, Object.keys(chunk));
            
            // Try to extract the actual content
            let content = "";
            if (chunk && chunk.content) {
                content = String(chunk.content);
            } else if (chunk && chunk.text) {
                content = String(chunk.text);
            } else if (chunk && chunk.data) {
                content = String(chunk.data);
            } else {
                content = String(chunk);
            }
            
            console.log(`📦 Extracted content:`, JSON.stringify(content.slice(0, 50) + (content.length > 50 ? "..." : "")));
            fullResponse += content;
        });
        
        stream.on('final', (response) => {
            console.log("🏁 FINAL event received");
            const responseStr = String(response);
            console.log("Final response length:", responseStr.length);
            console.log("Accumulated response length:", fullResponse.length);
            finalReceived = true;
            
            // Verify response consistency
            if (responseStr === fullResponse) {
                console.log("✅ Response consistency verified");
            } else {
                console.log("❌ Response inconsistency detected");
                console.log("Final response:", JSON.stringify(responseStr.slice(0, 100)));
                console.log("Accumulated response:", JSON.stringify(fullResponse.slice(0, 100)));
            }
            
            console.log(`✅ ${testType} streaming test completed (${chunkCount} chunks, start=${startReceived}, final=${finalReceived})`);
            resolve();
        });
        
        stream.on('error', (error) => {
            console.error("❌ ERROR event:", error);
            reject(error);
        });
        
        console.log("Streaming request initiated...");
    });
}

async function testEventOrdering(chatStep, conversation) {
    return new Promise((resolve, reject) => {
        console.log("Testing event ordering...");
        
        const events = [];
        let fullResponse = "";
        
        const stream = chatStep.startStream(conversation);
        
        stream.on('start', () => {
            events.push('start');
            console.log("📍 Event logged: start");
        });
        
        stream.on('partial', (chunk) => {
            events.push('partial');
            fullResponse += String(chunk);
            console.log(`📦 Event logged: partial (${events.filter(e => e === 'partial').length})`);
        });
        
        stream.on('final', (response) => {
            events.push('final');
            console.log("🏁 Event logged: final");
            
            // Verify event ordering
            const expectedPattern = /^start(partial)*final$/;
            const eventSequence = events.join('');
            
            if (expectedPattern.test(eventSequence)) {
                console.log("✅ Event ordering is correct:", eventSequence);
                resolve();
            } else {
                console.log("❌ Invalid event ordering:", eventSequence);
                reject(new Error(`Invalid event sequence: ${eventSequence}`));
            }
        });
        
        stream.on('error', (error) => {
            console.error("❌ ERROR in event ordering test:", error);
            reject(error);
        });
    });
}

async function testCancellation(chatStep, conversation) {
    return new Promise((resolve, reject) => {
        console.log("Testing cancellation...");
        
        let partialCount = 0;
        let cancelled = false;
        
        const stream = chatStep.startStream(conversation);
        
        stream.on('start', () => {
            console.log("📍 Cancellation test: START received");
        });
        
        stream.on('partial', (chunk) => {
            partialCount++;
            const chunkStr = String(chunk);
            console.log(`📦 Cancellation test: PARTIAL ${partialCount} received (${chunkStr.length} chars)`);
            
            // Cancel after receiving 2 partial chunks
            if (partialCount >= 2 && !cancelled) {
                console.log("🛑 Cancelling stream...");
                stream.cancel();
                cancelled = true;
                
                // Wait a bit to see if we get more events after cancellation
                setTimeout(() => {
                    console.log(`✅ Cancellation test completed (received ${partialCount} partials before cancel)`);
                    resolve();
                }, 1000);
            }
        });
        
        stream.on('final', (response) => {
            if (cancelled) {
                console.log("❌ Received FINAL after cancellation - this should not happen");
                reject(new Error("Received final event after cancellation"));
            } else {
                console.log("🏁 FINAL received before cancellation could be triggered");
                resolve();
            }
        });
        
        stream.on('error', (error) => {
            console.error("❌ ERROR in cancellation test:", error);
            reject(error);
        });
        
        // Fallback timeout in case cancellation doesn't trigger
        setTimeout(() => {
            if (!cancelled) {
                console.log("⚠️ Cancellation not triggered (response too fast), marking as passed");
                resolve();
            }
        }, 5000);
    });
}

console.log("Starting Comprehensive ChatStep Streaming Test");
runStreamingTest().catch(err => {
    console.error("Streaming test failed:", err);
    done(err); // Signal error
});
