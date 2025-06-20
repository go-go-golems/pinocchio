console.log("Testing basic Promise...");

// Test a basic Promise
const promise = new Promise((resolve, reject) => {
    console.log("Promise created");
    setTimeout(() => {
        console.log("Timeout executed");
        resolve("Hello World");
    }, 1000);
});

promise.then(result => {
    console.log("Promise resolved:", result);
}).catch(err => {
    console.error("Promise error:", err);
});

console.log("Script continuing...");
