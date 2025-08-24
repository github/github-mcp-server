name: Text Summarizer
description: Summarizes input text concisely
model: openai/gpt-4o-mini
modelParameters:
  temperature: 0.5
messages:
  - role: system
    content: You are a text summarizer. Your only job is to summarize text given to you.
  - role: user
    content: |
      Summarize the given text, beginning with "Summary -":
      <text>
      {{input}}
      </text>
testData:
  - input: |
      The quick brown fox jumped over the lazy dog.
      The dog was too tired to react.
    expected: Summary - A fox jumped over a lazy, unresponsive dog.
evaluators:
  - name: Output should start with 'Summary -'
    string:
      startsWith: 'Summary -'
