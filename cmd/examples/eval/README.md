I want an eval tool for my geppetto prompts:

Input: 
- eval dataset json file
- prompt template

Output:
- set of eval metrics

dataset + template -> llm calls -> compute accuracy -> eval results

# step 0

- [x] create a glazed command for evals
- [x] generate mock rows for eval results
- [x] wrap as command line tool

# step 1

- [x] load a eval data set from eval.json
  - array of objects
  - each object: 
    - input: hash[string]interface{}
    - golden answer: interface{}
    

- [ ] load a prompt from complaint.yaml
- [ ] iterate over each entry in eval.json
- [ ] interpolate the complaint.yaml command
- [ ] run it
- [ ] store the answer

go run ./cmd/eval --dataset eval.json --command complaint.yaml

# step 2

- run a grading function against the LLM answer
  - take a javascript script grading
- compute a accuracy score

go run ./cmd/eval --dataset eval.json --command complaint.yaml --scoring score.js

# step 3 

- REST API
- web ui (braintrust inspired)
  - import/export datasets
  - import/export/manage prompts
  - log + monitoring of testruns
  - streaming display of running datasets