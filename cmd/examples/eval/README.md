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

- [x] iterate over each entry in eval.json

- [x] load a prompt from complaint.yaml
- [x] interpolate the complaint.yaml command

### Running the actual LLM inference

- [x] run it
  - load the API key, etc...
  - create the chat step
  - get the step result
  - store the metadata in the result json

### Postprocessing the LLM response

- [x] store the answer
  - [ ] store the LLM metadata
  - [ ] store the date
  - [ ] give it a unique UUID

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

# features

- [ ] caching of inference