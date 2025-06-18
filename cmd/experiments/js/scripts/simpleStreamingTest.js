async function runSimpleStreamingTest() {
    console.log("=== Running Simple Streaming Test ===");
    
    // Create factory and step
    const factory = new DoubleStepFactory();
    const doubleStep = factory.newStep();
    
    console.log("Testing DoubleStep streaming with known result...");
    
    try {
        await testDoubleStepStreaming(doubleStep, 5.0);
    } catch (err) {
        console.error("DoubleStep streaming test failed:", err);
        done(err);
        return;
    }
    
    console.log("\n=== Simple streaming test completed successfully! ===");
    done();
}

function testDoubleStepStreaming(doubleStep, input) {
    return new Promise((resolve, reject) => {
        console.log("  Starting DoubleStep streaming test with input:", input);
        
        let hasStarted = false;
        let partialCount = 0;
        let finalResult = null;
        
        const stream = doubleStep.startStream(input);
        
        stream.on('start', () => {
            console.log("  ✓ 'start' event received");
            hasStarted = true;
        });
        
        stream.on('partial', (event) => {
            partialCount++;
            console.log(`  ✓ 'partial' event #${partialCount}:`, JSON.stringify(event));
        });
        
        stream.on('final', (event) => {
            finalResult = event;
            console.log("  ✓ 'final' event received:", JSON.stringify(event));
            
            // For DoubleStep, we expect the result to be input * 2
            const expectedResult = input * 2;
            
            // Verify event sequence
            if (!hasStarted) {
                reject(new Error("'start' event was not received"));
                return;
            }
            
            console.log("  ✓ DoubleStep streaming test passed!");
            console.log("    - Received", partialCount, "partial events");
            console.log("    - Expected result:", expectedResult);
            console.log("    - Actual result:", finalResult);
            resolve();
        });
        
        stream.on('error', (err) => {
            console.error("  ✗ Stream error:", err);
            reject(err);
        });
    });
}

console.log("Starting Simple Streaming Test");
runSimpleStreamingTest().catch(err => {
    console.error("Simple streaming test failed:", err);
    done(err);
});
