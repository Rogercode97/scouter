import { type Plugin, tool } from "@opencode-ai/plugin"
import { randomUUID } from 'crypto';

// Scouter — OpenCode plugin adapter
// This is a thin wrapper that registers Scouter's MCP tools in OpenCode.

export const ScouterPlugin: Plugin = async (ctx) => {
  return {
    tool: {
      "scouter-index": tool({
        description: "Analyze a file using the AST engine to index its structure (classes, methods, functions, variables).",
        args: {
          filePath: tool.schema.string().describe("The absolute path to the file to index."),
        },
        execute: async ({ filePath }) => {
           // The actual execution happens via the MCP server registered in opencode.json
           return JSON.stringify({ success: true, traceId: randomUUID() });
        }
      }),
      "scouter-read": tool({
        description: "Read a specific code scouterpet from a file using an AST pointer (byte-safe start/end positions).",
        args: {
          filePath: tool.schema.string().describe("The absolute path to the file."),
          pointer: tool.schema.unknown().describe("The AST pointer object containing position metadata."),
        },
        execute: async ({ filePath, pointer }) => {
           return JSON.stringify({ success: true, traceId: randomUUID() });
        }
      }),
    }
  }
}

export default ScouterPlugin
