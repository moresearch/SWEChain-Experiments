# SWEChain-Experiments


## Data Engineering


### Usage Examples for `swe_manager_task_distribution.go`

Below are several examples of how to run your SWE Manager Task Distribution script with various options:

---

**Basic Usage**

```sh
go run swe_manager_task_distribution.go -input data/tasks.csv
```
- Reads tasks from `data/tasks.csv`
- Uses default Ollama model and output directory

---

**Specify Ollama Model**

```sh
go run swe_manager_task_distribution.go -input data/tasks.csv -model llama3
```
- Uses the `llama3` model for categorization.

---

**Limit Number of Tasks (Random Subset)**

```sh
go run swe_manager_task_distribution.go -input data/tasks.csv -num_issues 50
```
- Randomly selects 50 issues from the available SWE Manager tasks for processing.

---

**Set Output Directory**

```sh
go run swe_manager_task_distribution.go -input data/tasks.csv -output ./out_agents
```
- Agent JSON files will be written to the `./out_agents` directory.

---

**Set Ollama API URL**

```sh
go run swe_manager_task_distribution.go -input data/tasks.csv -ollama_url http://localhost:11434/api/generate
```
- Uses a custom Ollama API endpoint.

---

**Increase LLM Retry Count**

```sh
go run swe_manager_task_distribution.go -input data/tasks.csv -llm_retries 5
```
- Retries up to 5 times per task if the LLM categorization fails.

---

**All Options Combined**

```sh
go run src/swe_manager_task_distribution.go \
  -input ./data/data.csv \
  -output ./data \
  -model granite3.3:8b \
  -ollama_url http://localhost:11434/api/generate \
  -num_issues 10 \
  -llm_retries 5


go run swe_manager_task_distribution.go \
  -input ./data/data.csv \
  -output ./data/agents \
  -model cogito:14b \
  -ollama_url http://localhost:11434/api/generate \
  -num_issues 4 \
  -num_agents 3 \
  -llm_retries 2
```

- Reads from `data/tasks.csv`
- Writes agent files to `./output_agents`
- Uses `llama3` model and custom API URL
- Randomly selects 100 tasks
- Retries LLM categorization up to 5 times per task

---

**Get Help**

```sh
go run swe_manager_task_distribution.go -h
```
- Prints all available flags and their usage.

---

**Note:**  
- The input CSV must have the expected columns (e.g. `variant`, `prompt`, etc.) and only SWE Manager tasks will be processed.
- The output directory will be created if it does not exist.




Speciality: ApplicationLogic
Description: This agent's speciality includes state management, client-side routing, form handling tasks.
Example: "When attempting to log in, the app doesn't switch focus to the first digit field after clicking 'didn't receive a magic code?'"

Speciality: ServerSideLogic
Description: API endpoints, DB queries, authentication, data processing-related tasks.
Example: The tooltip displays the users' email instead of their display name when hovering over the counter in the split preview.

Speciality: BugFixes
Description: This agent's speciality includes unexpected behaviours, errors, inconsistencies.
Example: Navigating back from the flag as offensive screen doesn't display the correct report page.

Speciality: UI/UX
Description: This agent's speciality includes design changes, layout, interaction improvements.
Example: Overlay background color is different.

Speciality: SystemWideQualityAndReliability
Description: This agent's speciality includes topics related to refactoring code, performance, optimization.
Example: Opening a thread calls the OpenReport API twice.

Speciality: NewFeatures or Enhancements
Description: This agent's speciality includes new functionality, optimization of existing features.
Example: Add the ability to mention yourself and use @here in the mentions auto-suggestions list.

Speciality: Reliability Improvements
Description: This agent's speciality includes logging, monitoring, testing.
Example: Add a character limit on the length of task names, room names, etc.



## Dashboard 

### Force-directed graph

### Disjoint force-directed graph

### Arc diagram

### Hexbin map

### Hierarchical edge bundling
