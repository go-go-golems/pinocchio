console.log("Testing setTimeout availability...");
console.log("typeof setTimeout:", typeof setTimeout);
console.log("typeof Promise:", typeof Promise);

if (typeof setTimeout !== 'undefined') {
    console.log("setTimeout is available");
    setTimeout(() => {
        console.log("setTimeout executed");
        done();
    }, 100);
} else {
    console.log("setTimeout is NOT available");
    done();
}
