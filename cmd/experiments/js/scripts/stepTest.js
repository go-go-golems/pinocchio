// Test Promise-based API
async function testPromise() {
    console.log("Testing Promise API...");
    try {
        const promise = doubleStep.startAsync(21)
        console.log("Promise created");
        const result = await promise;
        console.log("Promise result:", result[0]);
    } catch (err) {
        console.error("Promise error:", err);
    }
}

// Test blocking API
function testBlocking() {
    console.log("Testing Blocking API...");
    try {
        const result = doubleStep.startBlocking(32);
        console.log("Blocking result:", result[0]);
    } catch (err) {
        console.error("Blocking error:", err);
    }
}

// Test callback-based API
function testCallbacks() {
    console.log("Testing Callbacks API...");
    const cancel = doubleStep.startWithCallbacks(43, {
        onResult: (result) => console.log("Callback result:", result),
        onError: (err) => console.error("Callback error:", err),
        onDone: () => console.log("Callbacks complete"),
    });
}

// Run tests sequentially
async function runStepTests() {
    console.log("=== Running Step Tests ===");
    await testPromise();
    testBlocking();
    testCallbacks();
    console.log("Step tests complete");
}

runStepTests().catch(console.error); 