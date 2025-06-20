// Test error handling specifically
console.log("=== Testing Error Handling ===");

// Test error cases
try {
    console.log("Testing missing arguments...");
    // This should throw because no input provided
    doubleStep.startAsync();
} catch (err) {
    console.log("startAsync error (expected):", err.message || err);
}

try {
    console.log("Testing invalid callback...");
    // This should throw because second arg isn't a callback object
    doubleStep.startWithCallbacks(5, "not a callbacks object");
} catch (err) {
    console.log("startWithCallbacks error (expected):", err.message || err);
}

try {
    console.log("Testing missing callback argument...");
    // This should throw because no callbacks provided
    doubleStep.startWithCallbacks(5);
} catch (err) {
    console.log("startWithCallbacks missing arg error (expected):", err.message || err);
}

console.log("Error handling test complete");
done();
