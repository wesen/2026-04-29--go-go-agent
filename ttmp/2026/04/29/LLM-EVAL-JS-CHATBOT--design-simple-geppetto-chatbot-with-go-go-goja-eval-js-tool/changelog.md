# Changelog

## 2026-04-29

- Initial workspace created


## 2026-04-29

Created intern-facing design guide, investigation diary, source inventories, and evidence script for the Geppetto eval_js chatbot.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/LLM-EVAL-JS-CHATBOT--design-simple-geppetto-chatbot-with-go-go-goja-eval-js-tool/design-doc/01-geppetto-eval-js-chatbot-design-and-implementation-guide.md — Primary design deliverable
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/LLM-EVAL-JS-CHATBOT--design-simple-geppetto-chatbot-with-go-go-goja-eval-js-tool/reference/01-investigation-diary.md — Chronological investigation record
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/LLM-EVAL-JS-CHATBOT--design-simple-geppetto-chatbot-with-go-go-goja-eval-js-tool/scripts/01-inventory-and-evidence.sh — Reproducible evidence gathering script


## 2026-04-29

Validated ticket documentation, added vocabulary entries, and uploaded the design bundle to reMarkable at /ai/2026/04/29/LLM-EVAL-JS-CHATBOT.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/LLM-EVAL-JS-CHATBOT--design-simple-geppetto-chatbot-with-go-go-goja-eval-js-tool/changelog.md — Ticket changelog records delivery
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/LLM-EVAL-JS-CHATBOT--design-simple-geppetto-chatbot-with-go-go-goja-eval-js-tool/reference/01-investigation-diary.md — Updated with validation and reMarkable delivery evidence


## 2026-04-29

Added implementation tasks for the chat app scope: embedded help entries, inputDB/outputDB globals, eval_js, profile resolution, REPL, and tests.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/LLM-EVAL-JS-CHATBOT--design-simple-geppetto-chatbot-with-go-go-goja-eval-js-tool/reference/01-investigation-diary.md — Diary entry for implementation task planning
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/LLM-EVAL-JS-CHATBOT--design-simple-geppetto-chatbot-with-go-go-goja-eval-js-tool/tasks.md — Implementation task checklist


## 2026-04-29

Implemented the chat prototype with embedded help entries, inputDB/outputDB JavaScript globals, scopedjs eval_js, Pinocchio profile resolution, Geppetto runner wiring, REPL, and tests (commit 15de510).

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/main.go — chat command entrypoint
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/evaljs/runtime.go — eval_js runtime/tool implementation
- /home/manuel/code/wesen/2026-04-29--go-go-agent/internal/helpdb/helpdb.go — embedded help DB materialization


## 2026-04-29

Uploaded the refreshed implementation bundle to reMarkable at /ai/2026/04/29/LLM-EVAL-JS-CHATBOT.

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/LLM-EVAL-JS-CHATBOT--design-simple-geppetto-chatbot-with-go-go-goja-eval-js-tool/reference/01-investigation-diary.md — Records implementation bundle upload evidence


## 2026-04-29

Wired chat root logging/help like a Glazed command, changed REPL output to show eval_js args/results, and verified with gpt-5-nano-low in tmux (commit 9345f24).

### Related Files

- /home/manuel/code/wesen/2026-04-29--go-go-agent/cmd/chat/main.go — Glazed root setup and detailed turn printing
- /home/manuel/code/wesen/2026-04-29--go-go-agent/ttmp/2026/04/29/LLM-EVAL-JS-CHATBOT--design-simple-geppetto-chatbot-with-go-go-goja-eval-js-tool/sources/tmux-gpt5-nano-low-smoke-with-args.txt — Live tmux validation output

