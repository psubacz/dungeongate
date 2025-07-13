I started this project to test cluade code functionality. Here are a few thoughts:

- Requirements; Spend time working out what you want and what requirements need to be generated.
- Design; Ask a ton of questions when create a roadmap and ask for possiblities. having a understanding comp. engineering, alogrithms and data structures is useful if not required.
- Roadmap; Generate a roadmap per service is critical for multi-session working as it gives past, present, and future state of clauades work.
- Claude.md; At milestone intervlals regenerate the cluade.md to reflect the actual state of the repo and set expectations. Use it to set standards and conventions.
- Interrogation; use cluade to inspect the work done from a fresh prompt (no history). 
- .claude/settings.local.json; setting this up depends on your goals. this project was entirely vibecoded so more permissions were needed. I expect reads are better here.
- Testing; utilize the output of testing, linting, code coverage, audits to refine the roadmap.
- workflows; you can pip the output of something into claude such as `make tests | claude`
- troubleshooting; this is probably the issue with that gives teh biggest headache becuase you can find yourself in a loop that llm cant break out of. prompt engineering and direction are required to break out
- incorrect assumption; the LLM, claude in this case, will make assumptions about what to do. here is an example of a prompt:
    - scan the code for all instances of hardcode values and generate a todo list to fix. then 
- running on auto; its very tempting but you will eventually run into the `troubleshooting` issue above. this thing nneds to be watched like a sheep dog herds a flock
- longer term. there is an intereting issue here. 