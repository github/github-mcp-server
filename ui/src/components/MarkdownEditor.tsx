/**
 * MarkdownEditor component using GitHub's official @github/markdown-toolbar-element
 * with Primer React styling. This provides the same markdown editing experience
 * used on github.com.
 *
 * @see https://github.com/github/markdown-toolbar-element
 */
import { useId, useRef, useState, useEffect } from "react";
import { Text, Button, IconButton, useTheme } from "@primer/react";
import styles from "../styles.module.css";
import {
  BoldIcon,
  ItalicIcon,
  QuoteIcon,
  CodeIcon,
  LinkIcon,
  ListUnorderedIcon,
  ListOrderedIcon,
  TasklistIcon,
  MarkdownIcon,
} from "@primer/octicons-react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

// Import and register the web component
import "@github/markdown-toolbar-element";

// Declare types for the web component elements
declare module "react/jsx-runtime" {
  namespace JSX {
    interface IntrinsicElements {
      "markdown-toolbar": React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement> & { for: string },
        HTMLElement
      >;
      "md-bold": React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement>,
        HTMLElement
      >;
      "md-italic": React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement>,
        HTMLElement
      >;
      "md-quote": React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement>,
        HTMLElement
      >;
      "md-code": React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement>,
        HTMLElement
      >;
      "md-link": React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement>,
        HTMLElement
      >;
      "md-unordered-list": React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement>,
        HTMLElement
      >;
      "md-ordered-list": React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement>,
        HTMLElement
      >;
      "md-task-list": React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement>,
        HTMLElement
      >;
    }
  }
}

interface MarkdownEditorProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  minHeight?: number;
}

export function MarkdownEditor({
  value,
  onChange,
  placeholder = "Add a description...",
  minHeight = 150,
}: MarkdownEditorProps) {
  const textareaId = useId();
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [viewMode, setViewMode] = useState<"write" | "preview">("write");
  const { colorScheme } = useTheme();
  const isDark = colorScheme === "dark" || colorScheme === "dark_dimmed";

  // Sync external value changes to textarea
  useEffect(() => {
    if (textareaRef.current && textareaRef.current.value !== value) {
      textareaRef.current.value = value;
    }
  }, [value]);

  // Handle Enter key for list continuation
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key !== "Enter" || e.shiftKey) return;

    const textarea = textareaRef.current;
    if (!textarea) return;

    const { selectionStart, value: currentValue } = textarea;

    // Get the current line
    const beforeCursor = currentValue.substring(0, selectionStart);
    const lastNewline = beforeCursor.lastIndexOf("\n");
    const currentLine = beforeCursor.substring(lastNewline + 1);

    // Match different list patterns
    const unorderedMatch = currentLine.match(/^(\s*)([-*])\s/);
    const orderedMatch = currentLine.match(/^(\s*)(\d+)\.\s/);
    const taskMatch = currentLine.match(/^(\s*)([-*])\s\[[ x]\]\s/);

    let prefix = "";
    let isEmpty = false;

    if (taskMatch) {
      const indent = taskMatch[1];
      const marker = taskMatch[2];
      // Check if the line only has the list marker with no content
      isEmpty = currentLine.trim() === `${marker} [ ]` || currentLine.trim() === `${marker} [x]`;
      prefix = `${indent}${marker} [ ] `;
    } else if (orderedMatch) {
      const indent = orderedMatch[1];
      const num = parseInt(orderedMatch[2], 10);
      // Check if the line only has the list marker
      isEmpty = currentLine.trim() === `${num}.`;
      prefix = `${indent}${num + 1}. `;
    } else if (unorderedMatch) {
      const indent = unorderedMatch[1];
      const marker = unorderedMatch[2];
      // Check if the line only has the list marker
      isEmpty = currentLine.trim() === marker;
      prefix = `${indent}${marker} `;
    }

    if (prefix) {
      e.preventDefault();

      if (isEmpty) {
        // If just the list marker, remove it and exit list
        const newValue = currentValue.substring(0, lastNewline + 1) + currentValue.substring(selectionStart);
        onChange(newValue);
        // Set cursor position after React updates
        requestAnimationFrame(() => {
          if (textarea) {
            textarea.selectionStart = textarea.selectionEnd = lastNewline + 1;
            textarea.focus();
          }
        });
      } else {
        // Continue the list on the next line
        const afterCursor = currentValue.substring(selectionStart);
        const newValue = beforeCursor + "\n" + prefix + afterCursor;
        onChange(newValue);
        // Set cursor position after the prefix
        const newCursorPos = selectionStart + 1 + prefix.length;
        requestAnimationFrame(() => {
          if (textarea) {
            textarea.selectionStart = textarea.selectionEnd = newCursorPos;
            textarea.focus();
          }
        });
      }
    }
  };

  return (
    <div
      style={{
        borderWidth: 1,
        borderStyle: "solid",
        borderColor: "var(--borderColor-default)",
        borderRadius: 6,
        overflow: "hidden",
      }}
    >
      {/* Header with tabs and toolbar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          paddingLeft: 8,
          paddingRight: 8,
          paddingTop: 4,
          paddingBottom: 4,
          backgroundColor: "var(--bgColor-muted)",
          borderBottomWidth: 1,
          borderBottomStyle: "solid",
          borderBottomColor: "var(--borderColor-default)",
          overflow: "hidden",
        }}
      >
        {/* Write/Preview tabs */}
        <div style={{ display: "flex", flexShrink: 0, gap: 0 }}>
          <Button
            size="small"
            variant="invisible"
            onClick={() => setViewMode("write")}
            style={{
              fontWeight: viewMode === "write" ? 500 : 400,
              backgroundColor:
                viewMode === "write"
                  ? "var(--control-transparent-bgColor-hover)"
                  : "transparent",
              borderRadius: 6,
            }}
            className={styles.editorTab}
            data-active={viewMode === "write"}
          >
            Write
          </Button>
          <Button
            size="small"
            variant="invisible"
            onClick={() => setViewMode("preview")}
            style={{
              fontWeight: viewMode === "preview" ? 500 : 400,
              backgroundColor:
                viewMode === "preview"
                  ? "var(--control-transparent-bgColor-hover)"
                  : "transparent",
              borderRadius: 6,
            }}
            className={styles.editorTab}
            data-active={viewMode === "preview"}
          >
            Preview
          </Button>
        </div>

        {/* Toolbar - uses GitHub's official markdown-toolbar-element */}
        {viewMode === "write" && (
          <markdown-toolbar for={textareaId} style={{ display: "flex", overflow: "hidden", minWidth: 0, flexShrink: 1 }}>
            <div
              style={{
                display: "flex",
                gap: 0,
                alignItems: "center",
                overflowX: "auto",
              }}
            >
              <md-bold>
                <IconButton
                  icon={BoldIcon}
                  aria-label="Add bold text"
                  size="small"
                  variant="invisible"
                />
              </md-bold>
              <md-italic>
                <IconButton
                  icon={ItalicIcon}
                  aria-label="Add italic text"
                  size="small"
                  variant="invisible"
                />
              </md-italic>
              <md-quote>
                <IconButton
                  icon={QuoteIcon}
                  aria-label="Add a quote"
                  size="small"
                  variant="invisible"
                />
              </md-quote>
              <md-code>
                <IconButton
                  icon={CodeIcon}
                  aria-label="Add code"
                  size="small"
                  variant="invisible"
                />
              </md-code>
              <md-link>
                <IconButton
                  icon={LinkIcon}
                  aria-label="Add a link"
                  size="small"
                  variant="invisible"
                />
              </md-link>

              <div
                style={{
                  width: "1px",
                  height: 16,
                  backgroundColor: "var(--borderColor-default)",
                  marginLeft: 4,
                  marginRight: 4,
                }}
              />

              <md-unordered-list>
                <IconButton
                  icon={ListUnorderedIcon}
                  aria-label="Add a bulleted list"
                  size="small"
                  variant="invisible"
                />
              </md-unordered-list>
              <md-ordered-list>
                <IconButton
                  icon={ListOrderedIcon}
                  aria-label="Add a numbered list"
                  size="small"
                  variant="invisible"
                />
              </md-ordered-list>
              <md-task-list>
                <IconButton
                  icon={TasklistIcon}
                  aria-label="Add a task list"
                  size="small"
                  variant="invisible"
                />
              </md-task-list>
            </div>
          </markdown-toolbar>
        )}
      </div>

      {/* Content area */}
      {viewMode === "write" ? (
        <textarea
          ref={textareaRef}
          id={textareaId}
          defaultValue={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          style={{
            width: "100%",
            minHeight,
            padding: "12px",
            border: "none",
            resize: "vertical",
            fontFamily:
              '-apple-system, BlinkMacSystemFont, "Segoe UI", "Noto Sans", Helvetica, Arial, sans-serif',
            fontSize: "14px",
            lineHeight: 1.5,
            outline: "none",
            boxSizing: "border-box",
            backgroundColor: isDark ? "#0d1117" : "#ffffff",
            color: isDark ? "#e6edf3" : "#1f2328",
          }}
        />
      ) : (
        <div
          style={{
            backgroundColor: "var(--bgColor-default)",
            padding: "12px",
            minHeight,
            fontSize: 14,
            lineHeight: 1.5,
            color: "var(--fgColor-default)",
          }}
          className={styles.markdownPreview}
        >

          {value ? (
            <Markdown remarkPlugins={[remarkGfm]}>{value}</Markdown>
          ) : (
            <Text
              style={{ color: "var(--fgColor-muted)", fontStyle: "italic" }}
            >
              Nothing to preview
            </Text>
          )}
        </div>
      )}

      {/* Footer */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 1,
          paddingLeft: 8,
          paddingRight: 8,
          paddingTop: 4,
          paddingBottom: 4,
          backgroundColor: "var(--bgColor-muted)",
          borderTopWidth: 1,
          borderTopStyle: "solid",
          borderTopColor: "var(--borderColor-default)",
        }}
      >
        <MarkdownIcon size={16} />
        <Text style={{ fontSize: 12, color: "var(--fgColor-muted)" }}>
          Markdown is supported
        </Text>
      </div>
    </div>
  );
}
