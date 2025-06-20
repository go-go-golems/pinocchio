// Simple test to verify step APIs work
console.log("=== Simple Step API Test ===");

// Test Promise API
doubleStep.startAsync(5).then(result => {
    console.log("Promise result:", result[0]); // Should be 10
    
    // Test Blocking API
    console.log("Testing blocking...");
    const blockingResult = doubleStep.startBlocking(6);
    console.log("Blocking result:", blockingResult[0]); // Should be 12
    
    // Test Callbacks API
    console.log("Testing callbacks...");
    doubleStep.startWithCallbacks(7, {
        onResult: (result) => {
            console.log("Callback result:", result); // Should be 14
            console.log("All APIs working!");
            done();
        },
        onError: (err) => {
            console.error("Callback error:", err);
            done(err);
        }
    });
    
}).catch(err => {
    console.error("Promise failed:", err);
    done(err);
});
