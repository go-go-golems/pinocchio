function runTest1() {
    console.log("=== Running Watermill-based Streaming API Test ===");
    console.log("\n--- Test 1: Watermill-based streaming with doubleStep ---");
    
    try {
        let startReceived = false;
        let finalReceived = false;
        let errorReceived = false;
        let testComplete = false;
        
        const stepID = doubleStep.runWithEvents(5.0, function(event) {
            console.log("🔥 EVENT CALLBACK CALLED:", event.type);
            switch(event.type) {
                case "start":
                    console.log("✅ Start event received");
                    startReceived = true;
                    break;
                case "final":
                    console.log("✅ Final event received:", event.text);
                    finalReceived = true;
                    testComplete = true;
                    
                    // Check results when final event is received
                    console.log("Events received - Start:", startReceived, "Final:", finalReceived, "Error:", errorReceived);
                    
                    if (!startReceived || !finalReceived || errorReceived) {
                        console.error("Test 1 failed: Expected start and final events");
                        done(new Error("Expected start and final events, but got different results"));
                        return;
                    }
                    
                    console.log("✅ Test 1 PASSED!");
                    
                    // Test 1 complete, finish for now
                    done();
                    return;
                case "error":
                    console.log("❌ Error event received:", event.error);
                    errorReceived = true;
                    testComplete = true;
                    console.error("Test 1 failed: Received error event");
                    done(new Error("Received error event: " + event.error));
                    return;
                default:
                    console.log("⚠️ Unknown event type:", event.type);
            }
        });
        
        console.log("Step ID:", stepID);
        console.log("⏳ Waiting for events...");
        
    } catch (err) {
        console.error("Test 1 failed:", err);
        done(err);
        return;
    }
}

console.log("Starting Watermill-based Streaming API Test");
try {
    runTest1();
} catch (err) {
    console.error("Streaming test failed:", err);
    done(err);
}
