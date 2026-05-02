import argparse
import asyncio
import json
import re
import sys
import time
import traceback
import xml.etree.ElementTree as ET
from pathlib import Path
from typing import Any

# Modern Gemini SDK (Wave 12.0)
from google import genai
from google.genai import types as genai_types

# MCP SDK
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

EVALUATION_PROMPT = """You are an AI assistant with access to tools. When given a task, you MUST:
1. Use the available tools to complete the task
2. Provide summary of each step in your approach, wrapped in <summary> tags
3. Provide feedback on the tools provided, wrapped in <feedback> tags
4. Provide your final response, wrapped in <response> tags

Summary Requirements:
- In your <summary> tags, you must explain:
  - The steps you took to complete the task
  - Which tools you used, in what order, and why
  - The inputs you provided to each tool
  - The outputs you received from each tool
  - A summary for how you arrived at the response

Feedback Requirements:
- In your <feedback> tags, provide constructive feedback on the tools.

Response Requirements:
- Your response should be concise and directly address what was asked
- Always wrap your final response in <response> tags
- If you cannot solve the task return <response>NOT_FOUND</response>
- For names or text, provide the exact text requested
- Your response should go last"""

def parse_evaluation_file(file_path: Path) -> list[dict[str, Any]]:
    """Parse XML evaluation file with qa_pair elements."""
    try:
        tree = ET.parse(file_path)
        root = tree.getroot()
        evaluations = []
        for qa_pair in root.findall(".//qa_pair"):
            question_elem = qa_pair.find("question")
            answer_elem = qa_pair.find("answer")
            if question_elem is not None and answer_elem is not None:
                evaluations.append({
                    "question": (question_elem.text or "").strip(),
                    "answer": (answer_elem.text or "").strip(),
                })
        return evaluations
    except Exception as e:
        print(f"Error parsing evaluation file {file_path}: {e}")
        return []

def extract_xml_content(text: str, tag: str) -> str | None:
    """Extract content from XML tags."""
    pattern = rf"<{tag}>(.*?)</{tag}>"
    matches = re.findall(pattern, text, re.DOTALL)
    return matches[-1].strip() if matches else None

async def evaluate_single_task(
    client: genai.Client,
    model: str,
    qa_pair: dict[str, Any],
    session: ClientSession,
    task_index: int,
) -> dict[str, Any]:
    """Evaluate a single QA pair using built-in MCP support."""
    start_time = time.time()
    print(f"Task {task_index + 1}: {qa_pair['question']}")
    
    try:
        # The google-genai SDK handles the tool calling loop automatically
        # when we pass the ClientSession in the tools list.
        response = await client.aio.models.generate_content(
            model=model,
            contents=qa_pair["question"],
            config=genai_types.GenerateContentConfig(
                system_instruction=EVALUATION_PROMPT,
                tools=[session],
                temperature=0.0,
            )
        )
        
        final_text = response.text
        response_value = extract_xml_content(final_text, "response")
        summary = extract_xml_content(final_text, "summary")
        feedback = extract_xml_content(final_text, "feedback")
        
        duration_seconds = time.time() - start_time
        
        score = 0
        if response_value:
            actual = response_value.strip().lower()
            expected = qa_pair["answer"].strip().lower()
            if actual == expected or expected in actual:
                score = 1
                
        return {
            "question": qa_pair["question"],
            "expected": qa_pair["answer"],
            "actual": response_value,
            "score": score,
            "total_duration": duration_seconds,
            "summary": summary,
            "feedback": feedback,
        }
    except Exception as e:
        print(f"Error in task {task_index + 1}: {e}")
        return {
            "question": qa_pair["question"],
            "expected": qa_pair["answer"],
            "actual": f"ERROR: {str(e)}",
            "score": 0,
            "total_duration": time.time() - start_time,
            "summary": "N/A",
            "feedback": "N/A",
        }

REPORT_HEADER = """
# Evaluation Report (Gemini Native MCP)
## Summary
- **Accuracy**: {correct}/{total} ({accuracy:.1f}%)
- **Average Task Duration**: {average_duration_s:.2f}s
---
"""

TASK_TEMPLATE = """
### Task {task_num}
**Question**: {question}
**Ground Truth Answer**: `{expected_answer}`
**Actual Answer**: `{actual_answer}`
**Correct**: {correct_indicator}
**Duration**: {total_duration:.2f}s
**Summary**
{summary}
---
"""

async def main():
    parser = argparse.ArgumentParser(description="Evaluate Scouter MCP using Gemini Native Support")
    parser.add_argument("eval_file", type=Path)
    parser.add_argument("-c", "--command", default="./bin/scouter")
    parser.add_argument("-a", "--args", nargs="+", default=["mcp"])
    parser.add_argument("-m", "--model", default="gemini-2.0-flash")
    parser.add_argument("-o", "--output", type=Path)

    args = parser.parse_args()

    if not args.eval_file.exists():
        print(f"Error: {args.eval_file} not found")
        return

    # Initialize Gemini Client
    client = genai.Client()

    # MCP Server Parameters
    server_params = StdioServerParameters(
        command=args.command,
        args=args.args,
    )

    print(f"🔗 Connecting to MCP server: {args.command} {' '.join(args.args)}")
    
    async with stdio_client(server_params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()
            print("✅ MCP Session Initialized")
            
            qa_pairs = parse_evaluation_file(args.eval_file)
            results = []
            
            for i, qa_pair in enumerate(qa_pairs):
                res = await evaluate_single_task(client, args.model, qa_pair, session, i)
                results.append(res)
                
            correct = sum(r["score"] for r in results)
            total = len(results)
            accuracy = (correct / total) * 100 if total > 0 else 0
            avg_duration = sum(r["total_duration"] for r in results) / total if total > 0 else 0
            
            report = REPORT_HEADER.format(
                correct=correct,
                total=total,
                accuracy=accuracy,
                average_duration_s=avg_duration
            )
            
            for i, res in enumerate(results):
                report += TASK_TEMPLATE.format(
                    task_num=i+1,
                    question=res["question"],
                    expected_answer=res["expected"],
                    actual_answer=res["actual"] or "N/A",
                    correct_indicator="✅" if res["score"] else "❌",
                    total_duration=res["total_duration"],
                    summary=res["summary"] or "N/A"
                )
            
            if args.output:
                args.output.write_text(report)
                print(f"\n✅ Report saved to {args.output}")
            else:
                print("\n" + report)

if __name__ == "__main__":
    asyncio.run(main())
