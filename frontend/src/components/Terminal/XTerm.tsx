import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { EventsOn, EventsOff } from "../../../wailsjs/runtime/runtime";

interface XTermProps {
  id: string; // Session ID
  onData: (data: string) => void; // Callback when user types
  onResize: (rows: number, cols: number) => void;
  className?: string;
}

export function XTerm({ id, onData, onResize, className }: XTermProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);

  useEffect(() => {
    if (!containerRef.current) return;

    // Initialize Terminal
    const term = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      theme: {
        background: "#1e1e1e",
        foreground: "#f0f0f0",
      },
      allowProposedApi: true,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    term.open(containerRef.current);
    fitAddon.fit();

    terminalRef.current = term;
    fitAddonRef.current = fitAddon;

    // Report initial size
    onResize(term.rows, term.cols);

    // Focus terminal
    term.focus();

    // Handle user input
    const disposeOnData = term.onData((data) => {
      onData(data);
    });

    // Handle resize
    const handleResize = () => {
      if (containerRef.current && fitAddonRef.current) {
        fitAddonRef.current.fit();
        if (terminalRef.current) {
          onResize(terminalRef.current.rows, terminalRef.current.cols);
        }
      }
    };

    // ResizeObserver for container
    const resizeObserver = new ResizeObserver(() => {
      handleResize();
    });
    resizeObserver.observe(containerRef.current);

    window.addEventListener("resize", handleResize);

    // Listen for incoming data from backend
    // Data comes as Base64 string from backend to handle binary safely
    EventsOn(`terminal:data:${id}`, (b64Data: string) => {
      try {
        const data = atob(b64Data);
        term.write(data);
      } catch (e) {
        console.error("Failed to decode terminal data", e);
      }
    });

    EventsOn(`terminal:closed:${id}`, () => {
      term.writeln("\r\n[Process completed]\r\n");
    });

    return () => {
      disposeOnData.dispose();
      window.removeEventListener("resize", handleResize);
      resizeObserver.disconnect();
      term.dispose();
      // EventsOff(eventName) // Wails runtime usually returns a cleanup function or we might need to manually off?
      // Checking runtime.js usually EventsOn returns nothing, need to check implementation.
      // If it doesn't return cleanup, we might leak listeners if we don't call EventsOff.
      EventsOff(`terminal:data:${id}`);
      EventsOff(`terminal:closed:${id}`);
    };
  }, [id, onData, onResize]);

  return <div ref={containerRef} className={`w-full h-full ${className}`} />;
}
