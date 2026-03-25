package context

import "github.com/rishav1305/soul/internal/chat/stream"

func sentinelContext() ProductContext {
	return ProductContext{
		System: `You are connected to Soul Sentinel, an LLM security training platform. Sentinel provides capture-the-flag challenges where users learn to attack and defend LLM systems.

Key capabilities:
- Browse and start CTF challenges across difficulty levels (beginner, mid, advanced).
- Submit flags to complete challenges and track progress.
- Attack LLM systems in challenge mode or a configurable sandbox.
- Configure sandbox AI targets with custom system prompts and weakness levels.
- Define defense rules to harden AI systems against prompt injection.
- Scan other Soul products for security vulnerabilities.

Guide users through challenges, explain attack techniques, and help them understand LLM security concepts.`,
		Tools: []stream.Tool{
			{
				Name:        "challenge_list",
				Description: "List available CTF challenges, optionally filtered by category or difficulty.",
				InputSchema: mustJSON(`{"type":"object","properties":{"category":{"type":"string","description":"Filter by challenge category"},"difficulty":{"type":"string","enum":["beginner","mid","advanced"],"description":"Filter by difficulty level"}}}`),
			},
			{
				Name:        "challenge_start",
				Description: "Start a CTF challenge. Returns the challenge briefing and initial context.",
				InputSchema: mustJSON(`{"type":"object","properties":{"challenge_id":{"type":"string","description":"ID of the challenge to start"},"reset":{"type":"boolean","description":"Reset progress if challenge was previously started"}},"required":["challenge_id"]}`),
			},
			{
				Name:        "challenge_submit",
				Description: "Submit a flag to complete a challenge.",
				InputSchema: mustJSON(`{"type":"object","properties":{"challenge_id":{"type":"string","description":"ID of the challenge"},"flag":{"type":"string","description":"The captured flag value"}},"required":["challenge_id","flag"]}`),
			},
			{
				Name:        "attack",
				Description: "Send an attack payload to an LLM target in challenge or sandbox mode.",
				InputSchema: mustJSON(`{"type":"object","properties":{"mode":{"type":"string","enum":["challenge","sandbox"],"description":"Attack mode: challenge (against a CTF target) or sandbox (against a custom target)"},"challenge_id":{"type":"string","description":"Challenge ID (required for challenge mode)"},"payload":{"type":"string","description":"The attack payload to send to the target LLM"}},"required":["mode","payload"]}`),
			},
			{
				Name:        "sandbox_config",
				Description: "Configure a sandbox AI target with a custom system prompt and weakness level.",
				InputSchema: mustJSON(`{"type":"object","properties":{"name":{"type":"string","description":"Name for the sandbox target"},"system_prompt":{"type":"string","description":"System prompt for the target LLM"},"weakness_level":{"type":"string","enum":["none","low","medium","high"],"description":"How vulnerable the target is to prompt injection"}},"required":["name","system_prompt"]}`),
			},
			{
				Name:        "defend",
				Description: "Add a defense rule to harden an AI system against prompt injection attacks.",
				InputSchema: mustJSON(`{"type":"object","properties":{"name":{"type":"string","description":"Name of the defense rule"},"rule":{"type":"string","description":"The defense rule specification"}},"required":["name","rule"]}`),
			},
			{
				Name:        "scan",
				Description: "Scan a Soul product for LLM security vulnerabilities.",
				InputSchema: mustJSON(`{"type":"object","properties":{"product":{"type":"string","description":"Product to scan (e.g., tasks, tutor, projects)"}},"required":["product"]}`),
			},
		},
	}
}
