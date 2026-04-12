import { useState, useEffect, useRef } from "react";
import {
  ArrowUpDown,
  ChevronUp,
  ChevronDown,
  Search,
  HelpCircle,
} from "lucide-react";
import {
  GetTimelineEvents,
  GetSnapshots,
  GetSnapshotDiff,
  TakeSnapshot,
  IsTimelineAvailable,
  ListResources,
} from "../../../wailsjs/go/main/App";

const REFRESH_BTN_CLASS =
  "px-3 py-1.5 text-sm bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-200 rounded-lg transition-colors disabled:opacity-50";

interface TimelineEvent {
  id: number;
  cluster: string;
  namespace: string;
  kind: string;
  name: string;
  type: string;
  reason: string;
  message: string;
  firstSeen: string;
  lastSeen: string;
  count: number;
  sourceComponent: string;
}

interface SnapshotInfo {
  timestamp: string;
}

interface SnapshotDiff {
  before: string;
  after: string;
  added: { kind: string; name: string; namespace: string }[];
  removed: { kind: string; name: string; namespace: string }[];
  modified: {
    kind: string;
    name: string;
    namespace: string;
    oldStatus?: string;
    newStatus?: string;
  }[];
}

export function TimelineView({
  isConnected,
  namespaces,
}: {
  isConnected: boolean;
  namespaces: string[];
}) {
  const [events, setEvents] = useState<TimelineEvent[]>([]);
  const [snapshots, setSnapshots] = useState<SnapshotInfo[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [namespaceFilter, setNamespaceFilter] = useState("");
  const [searchText, setSearchText] = useState("");
  const [typeFilter, setTypeFilter] = useState<"" | "Warning" | "Normal">("");
  const [timeRange, setTimeRange] = useState<number>(60); // minutes
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [mode, setMode] = useState<"events" | "snapshots" | "diff">("events");
  const [selectedSnapshots, setSelectedSnapshots] = useState<string[]>([]);
  const [diff, setDiff] = useState<SnapshotDiff | null>(null);
  const [timelineAvailable, setTimelineAvailable] = useState(true);
  const [takingSnapshot, setTakingSnapshot] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState<number>(-1);
  const tableRef = useRef<HTMLDivElement>(null);
  const filterRef = useRef<HTMLInputElement>(null);
  const [showTimeRangeDropdown, setShowTimeRangeDropdown] = useState(false);
  const [showInfoTooltip, setShowInfoTooltip] = useState(false);

  // Sorting for events
  const [eventSortField, setEventSortField] = useState<
    "time" | "type" | "reason" | "object" | "count"
  >("time");
  const [eventSortDirection, setEventSortDirection] = useState<"asc" | "desc">(
    "desc"
  );

  const handleEventSort = (
    field: "time" | "type" | "reason" | "object" | "count"
  ) => {
    if (eventSortField === field) {
      setEventSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setEventSortField(field);
      setEventSortDirection(field === "time" ? "desc" : "asc");
    }
  };

  const EventSortIndicator = ({ field }: { field: string }) => {
    if (eventSortField !== field) {
      return (
        <ArrowUpDown size={14} className="text-slate-400 dark:text-slate-600" />
      );
    }
    return eventSortDirection === "asc" ? (
      <ChevronUp size={14} className="text-accent-500 dark:text-accent-400" />
    ) : (
      <ChevronDown size={14} className="text-accent-500 dark:text-accent-400" />
    );
  };

  const fetchEvents = async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await GetTimelineEvents({
        namespace: namespaceFilter,
        type: typeFilter,
        sinceMinutes: timeRange,
        limit: 500,
        kind: "",
      });
      setEvents(result || []);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (msg.includes("not available") || msg.includes("not initialized")) {
        setTimelineAvailable(false);
        // Fallback to ListResources
        try {
          const fallback = await ListResources("events", namespaceFilter);
          setEvents(
            (fallback || []).map((e: any, i: number) => ({
              id: i,
              cluster: "",
              namespace: e.namespace,
              kind: "Event",
              name: e.name,
              type: e.status || "Normal",
              reason: e.labels?.reason || "",
              message: e.labels?.message || "",
              firstSeen: "",
              lastSeen: e.age,
              count: 1,
              sourceComponent: "",
            }))
          );
        } catch {
          setError("Failed to fetch events");
        }
      } else {
        setError(msg);
      }
    } finally {
      setLoading(false);
    }
  };

  const fetchSnapshots = async () => {
    try {
      const result = await GetSnapshots(50);
      setSnapshots(result || []);
    } catch (err) {
      console.error("Failed to fetch snapshots:", err);
    }
  };

  const handleCompare = async () => {
    if (selectedSnapshots.length !== 2) return;
    setLoading(true);
    try {
      const result = await GetSnapshotDiff(
        selectedSnapshots[0],
        selectedSnapshots[1]
      );
      setDiff(result);
      setMode("diff");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to compare snapshots"
      );
    } finally {
      setLoading(false);
    }
  };

  const handleTakeSnapshot = async () => {
    setTakingSnapshot(true);
    try {
      await TakeSnapshot();
      await fetchSnapshots();
    } catch (err) {
      console.error("Failed to take snapshot:", err);
    } finally {
      setTakingSnapshot(false);
    }
  };

  useEffect(() => {
    const checkAvailable = async () => {
      try {
        const available = await IsTimelineAvailable();
        setTimelineAvailable(available);
      } catch {
        setTimelineAvailable(false);
      }
    };
    checkAvailable();
  }, []);

  useEffect(() => {
    if (isConnected) {
      fetchEvents();
      if (timelineAvailable) fetchSnapshots();
    } else {
      setEvents([]);
      setSnapshots([]);
    }
  }, [isConnected, namespaceFilter, typeFilter, timeRange, timelineAvailable]);

  useEffect(() => {
    if (!autoRefresh || !isConnected) return;
    const interval = setInterval(fetchEvents, 5000);
    return () => clearInterval(interval);
  }, [autoRefresh, isConnected, namespaceFilter, typeFilter, timeRange]);

  // Reset selection when events change
  useEffect(() => {
    setSelectedIndex(-1);
  }, [events.length, namespaceFilter, typeFilter]);

  // Keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.isContentEditable
      ) {
        return;
      }

      switch (e.key) {
        case "/":
          e.preventDefault();
          filterRef.current?.focus();
          break;
        case "r":
          if (!loading && isConnected) {
            e.preventDefault();
            fetchEvents();
          }
          break;
        case "e":
          e.preventDefault();
          setMode("events");
          break;
        case "w":
          e.preventDefault();
          setTypeFilter((prev) => (prev === "Warning" ? "" : "Warning"));
          break;
        case "ArrowDown":
        case "j":
          if (mode !== "events") return;
          e.preventDefault();
          setSelectedIndex((prev) =>
            prev < events.length - 1 ? prev + 1 : prev
          );
          break;
        case "ArrowUp":
        case "k":
          if (mode !== "events") return;
          e.preventDefault();
          setSelectedIndex((prev) => (prev > 0 ? prev - 1 : 0));
          break;
      }

      // Shift+letter sorting shortcuts for events
      if (e.shiftKey && mode === "events") {
        switch (e.key) {
          case "T": // Time
            e.preventDefault();
            handleEventSort("time");
            break;
          case "Y": // tYpe
            e.preventDefault();
            handleEventSort("type");
            break;
          case "R": // Reason
            e.preventDefault();
            handleEventSort("reason");
            break;
          case "O": // Object
            e.preventDefault();
            handleEventSort("object");
            break;
          case "C": // Count
            e.preventDefault();
            handleEventSort("count");
            break;
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [events.length, mode, loading, isConnected, eventSortField]);

  // Scroll selected row into view
  useEffect(() => {
    if (selectedIndex >= 0 && tableRef.current) {
      const row = tableRef.current.querySelector(
        `tr[data-index="${selectedIndex}"]`
      );
      row?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [selectedIndex]);

  const getTypeColor = (type: string) => {
    if (type === "Warning") return "text-yellow-500 dark:text-yellow-400";
    return "text-emerald-500 dark:text-emerald-400";
  };

  const getTypeIcon = (type: string) => {
    if (type === "Warning") return "⚠";
    return "ℹ";
  };

  const formatTime = (isoString: string) => {
    if (!isoString) return "-";
    try {
      return new Date(isoString).toLocaleTimeString();
    } catch {
      return isoString;
    }
  };

  const toggleSnapshotSelection = (ts: string) => {
    setSelectedSnapshots((prev) => {
      if (prev.includes(ts)) return prev.filter((s) => s !== ts);
      if (prev.length >= 2) return [prev[1], ts];
      return [...prev, ts];
    });
  };

  return (
    <div className="h-full flex flex-col">
      {/* Mode tabs */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex gap-2">
          <button
            onClick={() => setMode("events")}
            className={`px-4 py-2 text-sm rounded-lg border transition-colors ${
              mode === "events"
                ? "bg-accent-500/20 border-accent-500/50 text-accent-600 dark:text-accent-400"
                : "bg-white dark:bg-slate-800/50 border-stone-200 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/50 text-stone-600 dark:text-slate-400"
            }`}
          >
            Events
          </button>
          {timelineAvailable && (
            <button
              onClick={() => {
                setMode("snapshots");
                fetchSnapshots();
              }}
              className={`px-4 py-2 text-sm rounded-lg border transition-colors ${
                mode === "snapshots" || mode === "diff"
                  ? "bg-accent-500/20 border-accent-500/50 text-accent-600 dark:text-accent-400"
                  : "bg-white dark:bg-slate-800/50 border-stone-200 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/50 text-stone-600 dark:text-slate-400"
              }`}
            >
              Snapshots
            </button>
          )}
        </div>
        {!timelineAvailable && (
          <span className="text-xs text-yellow-600 dark:text-yellow-400">
            ⚠ History DB not available - showing live events only
          </span>
        )}
      </div>

      {mode === "events" && (
        <>
          <div className="flex items-center justify-between mb-4 flex-wrap gap-3">
            <div className="flex gap-3 items-center flex-wrap">
              <div className="relative">
                <select
                  value={namespaceFilter}
                  onChange={(e) => setNamespaceFilter(e.target.value)}
                  className="w-48 appearance-none bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50 text-stone-800 dark:text-slate-100 pr-8"
                >
                  <option value="">All Namespaces</option>
                  {namespaces.map((ns) => (
                    <option key={ns} value={ns}>
                      {ns}
                    </option>
                  ))}
                </select>
                <div className="absolute inset-y-0 right-0 flex items-center px-2 pointer-events-none">
                  <ChevronDown
                    size={14}
                    className="text-slate-400 dark:text-slate-500"
                  />
                </div>
              </div>

              <div className="relative">
                <input
                  ref={filterRef}
                  type="text"
                  placeholder="Search events... (/)"
                  value={searchText}
                  onChange={(e) => setSearchText(e.target.value)}
                  className="w-64 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg pl-8 pr-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50 text-stone-800 dark:text-slate-100 placeholder-stone-400 dark:placeholder-slate-500"
                />
                <Search
                  size={14}
                  className="absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400"
                />
              </div>
              <div className="flex gap-1">
                {(
                  [
                    ["", "All"],
                    ["Warning", "Warning (w)"],
                    ["Normal", "Normal"],
                  ] as const
                ).map(([value, label]) => (
                  <button
                    key={value}
                    onClick={() => setTypeFilter(value)}
                    className={`px-3 py-1.5 text-sm rounded-lg border transition-colors ${
                      typeFilter === value
                        ? "bg-accent-500/20 border-accent-500/50 text-accent-600 dark:text-accent-400"
                        : "bg-white dark:bg-slate-800/50 border-stone-200 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/50 text-stone-600 dark:text-slate-400"
                    }`}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <div className="relative">
                <button
                  onClick={() =>
                    setShowTimeRangeDropdown(!showTimeRangeDropdown)
                  }
                  className="flex items-center gap-2 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-sm hover:border-stone-300 dark:hover:border-slate-600 transition-colors text-stone-800 dark:text-slate-100 shadow-sm dark:shadow-none"
                >
                  <span>
                    {timeRange === 5
                      ? "Last 5 minutes"
                      : timeRange === 15
                      ? "Last 15 minutes"
                      : timeRange === 30
                      ? "Last 30 minutes"
                      : timeRange === 60
                      ? "Last 1 hour"
                      : timeRange === 360
                      ? "Last 6 hours"
                      : timeRange === 1440
                      ? "Last 24 hours"
                      : "Last 7 days"}
                  </span>
                  <ChevronDown
                    size={14}
                    className={`text-slate-400 dark:text-slate-500 transition-transform ${
                      showTimeRangeDropdown ? "rotate-180" : ""
                    }`}
                  />
                </button>
                {showTimeRangeDropdown && (
                  <div className="absolute right-0 z-50 mt-1 w-48 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg shadow-lg overflow-hidden">
                    {[
                      { value: 5, label: "Last 5 minutes" },
                      { value: 15, label: "Last 15 minutes" },
                      { value: 30, label: "Last 30 minutes" },
                      { value: 60, label: "Last 1 hour" },
                      { value: 360, label: "Last 6 hours" },
                      { value: 1440, label: "Last 24 hours" },
                      { value: 10080, label: "Last 7 days" },
                    ].map((opt) => (
                      <button
                        key={opt.value}
                        onClick={() => {
                          setTimeRange(opt.value);
                          setShowTimeRangeDropdown(false);
                        }}
                        className={`w-full px-3 py-2 text-sm text-left transition-colors ${
                          timeRange === opt.value
                            ? "bg-accent-500/10 text-accent-600 dark:text-accent-400"
                            : "text-stone-600 dark:text-slate-300 hover:bg-stone-100 dark:hover:bg-slate-700"
                        }`}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                )}
              </div>
              <div className="relative">
                <button
                  onMouseEnter={() => setShowInfoTooltip(true)}
                  onMouseLeave={() => setShowInfoTooltip(false)}
                  className="p-1.5 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
                >
                  <HelpCircle size={16} />
                </button>
                {showInfoTooltip && (
                  <div className="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-64 bg-slate-800 text-white text-xs rounded-lg p-3 shadow-xl z-50 pointer-events-none">
                    <div className="absolute bottom-[-4px] left-1/2 -translate-x-1/2 w-2 h-2 bg-slate-800 rotate-45"></div>
                    <p className="font-medium mb-1 text-emerald-400">
                      Extended History
                    </p>
                    <p className="leading-relaxed text-slate-300">
                      Kubecat archives events for 7 days (vs. standard 1h),
                      allowing you to troubleshoot issues that happened while
                      you were away.
                    </p>
                  </div>
                )}
              </div>
            </div>
            <div className="flex items-center gap-3">
              <label className="flex items-center gap-2 text-sm text-slate-400">
                <input
                  type="checkbox"
                  checked={autoRefresh}
                  onChange={(e) => setAutoRefresh(e.target.checked)}
                  className="rounded bg-slate-700 border-slate-600"
                />
                Auto-refresh
              </label>
              <button
                onClick={fetchEvents}
                disabled={loading || !isConnected}
                className={REFRESH_BTN_CLASS}
              >
                Refresh
              </button>
            </div>
          </div>

          <div className="flex-1 window-glass rounded-xl overflow-hidden shadow-sm dark:shadow-none transition-colors">
            {!isConnected ? (
              <p className="text-slate-500 dark:text-slate-400 text-center py-12">
                Connect to a cluster to view events
              </p>
            ) : loading && events.length === 0 ? (
              <div className="flex items-center justify-center py-12">
                <div className="w-6 h-6 border-2 border-accent-500 dark:border-accent-400 border-t-transparent rounded-full animate-spin" />
              </div>
            ) : error ? (
              <p className="text-red-500 dark:text-red-400 text-center py-12">
                {error}
              </p>
            ) : events.length === 0 ? (
              <p className="text-slate-500 dark:text-slate-400 text-center py-12">
                No events found
              </p>
            ) : (
              <div ref={tableRef} className="overflow-auto h-full">
                <table className="w-full text-sm table-fixed">
                  <thead className="bg-stone-50 dark:bg-slate-900 sticky top-0 z-10 transition-colors shadow-sm">
                    <tr className="text-left text-stone-500 dark:text-slate-400">
                      <th
                        className="px-4 py-3 font-medium w-24 cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none"
                        onClick={() => handleEventSort("time")}
                      >
                        <span className="flex items-center gap-1">
                          Time <EventSortIndicator field="time" />
                        </span>
                      </th>
                      <th
                        className="px-4 py-3 font-medium w-16 cursor-pointer hover:bg-slate-800 transition-colors select-none"
                        onClick={() => handleEventSort("type")}
                      >
                        <span className="flex items-center gap-1">
                          Type <EventSortIndicator field="type" />
                        </span>
                      </th>
                      <th
                        className="px-4 py-3 font-medium w-32 xl:w-48 cursor-pointer hover:bg-slate-800 transition-colors select-none"
                        onClick={() => handleEventSort("reason")}
                      >
                        <span className="flex items-center gap-1">
                          Reason <EventSortIndicator field="reason" />
                        </span>
                      </th>
                      <th
                        className="px-4 py-3 font-medium w-48 xl:w-64 cursor-pointer hover:bg-slate-800 transition-colors select-none"
                        onClick={() => handleEventSort("object")}
                      >
                        <span className="flex items-center gap-1">
                          Object <EventSortIndicator field="object" />
                        </span>
                      </th>
                      <th className="px-4 py-3 font-medium">Message</th>
                      {timelineAvailable && (
                        <th
                          className="px-4 py-3 font-medium w-16 cursor-pointer hover:bg-slate-800 transition-colors select-none"
                          onClick={() => handleEventSort("count")}
                        >
                          <span className="flex items-center gap-1">
                            Count <EventSortIndicator field="count" />
                          </span>
                        </th>
                      )}
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-stone-200 dark:divide-slate-700/50">
                    {[...events]
                      .filter((e) => {
                        if (!searchText) return true;
                        const term = searchText.toLowerCase();
                        return (
                          e.message?.toLowerCase().includes(term) ||
                          e.reason?.toLowerCase().includes(term) ||
                          e.name?.toLowerCase().includes(term) ||
                          e.kind?.toLowerCase().includes(term)
                        );
                      })
                      .sort((a, b) => {
                        let comparison = 0;
                        switch (eventSortField) {
                          case "time":
                            comparison = a.lastSeen.localeCompare(b.lastSeen);
                            break;
                          case "type":
                            comparison = a.type.localeCompare(b.type);
                            break;
                          case "reason":
                            comparison = (a.reason || "").localeCompare(
                              b.reason || ""
                            );
                            break;
                          case "object":
                            comparison =
                              `${a.namespace}/${a.kind}/${a.name}`.localeCompare(
                                `${b.namespace}/${b.kind}/${b.name}`
                              );
                            break;
                          case "count":
                            comparison = a.count - b.count;
                            break;
                        }
                        return eventSortDirection === "asc"
                          ? comparison
                          : -comparison;
                      })
                      .map((event, idx) => (
                        <tr
                          key={event.id}
                          data-index={idx}
                          className={`transition-colors text-xs ${
                            selectedIndex === idx
                              ? "bg-accent-500/20 ring-1 ring-accent-500/50"
                              : "hover:bg-stone-50 dark:hover:bg-slate-700/30"
                          }`}
                        >
                          <td className="px-4 py-3 text-stone-500 dark:text-slate-400 font-mono whitespace-nowrap">
                            {formatTime(event.lastSeen)}
                          </td>
                          <td
                            className={`px-4 py-3 ${getTypeColor(event.type)}`}
                          >
                            {getTypeIcon(event.type)}
                          </td>
                          <td
                            className="px-4 py-3 font-medium text-stone-700 dark:text-slate-200 truncate"
                            title={event.reason}
                          >
                            {event.reason || "-"}
                          </td>
                          <td
                            className="px-4 py-3 font-mono text-accent-600 dark:text-accent-400 truncate"
                            title={`${event.namespace}/${event.kind}/${event.name}`}
                          >
                            <span className="text-stone-400 dark:text-slate-500">
                              {event.namespace}/
                            </span>
                            <span className="text-stone-500 dark:text-slate-400">
                              {event.kind}/
                            </span>
                            {event.name}
                          </td>
                          <td
                            className="px-4 py-3 text-stone-600 dark:text-slate-300 truncate"
                            title={event.message}
                          >
                            {event.message || "-"}
                          </td>
                          {timelineAvailable && (
                            <td className="px-4 py-3 text-stone-500 dark:text-slate-400 text-right">
                              {event.count > 1 ? `\u00d7${event.count}` : ""}
                            </td>
                          )}
                        </tr>
                      ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          <div className="mt-3 text-sm text-slate-500 dark:text-slate-400">
            {events.length} events
            {autoRefresh && (
              <span className="ml-2 text-emerald-600 dark:text-emerald-400">
                ● Auto-refreshing
              </span>
            )}
          </div>
        </>
      )}

      {mode === "snapshots" && (
        <>
          <div className="flex items-center justify-between mb-4">
            <p className="text-sm text-slate-600 dark:text-slate-400">
              Select two snapshots to compare. {selectedSnapshots.length}/2
              selected.
            </p>
            <div className="flex gap-2">
              <button
                onClick={handleTakeSnapshot}
                disabled={takingSnapshot || !isConnected}
                className="px-3 py-1.5 text-sm bg-accent-500 hover:bg-accent-600 text-white rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
              >
                {takingSnapshot && (
                  <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                )}
                Take Snapshot
              </button>
              <button
                onClick={handleCompare}
                disabled={selectedSnapshots.length !== 2 || loading}
                className="px-3 py-1.5 text-sm bg-emerald-600 hover:bg-emerald-700 text-white rounded-lg transition-colors disabled:opacity-50"
              >
                Compare
              </button>
            </div>
          </div>

          <div className="flex-1 window-glass rounded-xl overflow-auto transition-colors">
            {snapshots.length === 0 ? (
              <p className="text-slate-400 text-center py-12">
                No snapshots yet. Click "Take Snapshot" to create one.
              </p>
            ) : (
              <div className="p-4 space-y-2">
                {snapshots.map((snap, idx) => {
                  const isSelected = selectedSnapshots.includes(snap.timestamp);
                  const selectionIndex = selectedSnapshots.indexOf(
                    snap.timestamp
                  );
                  return (
                    <button
                      key={idx}
                      onClick={() => toggleSnapshotSelection(snap.timestamp)}
                      className={`w-full text-left px-4 py-3 rounded-lg border transition-colors flex items-center justify-between ${
                        isSelected
                          ? "bg-accent-500/20 border-accent-500/50"
                          : "bg-slate-50 dark:bg-slate-900/50 border-slate-200 dark:border-slate-700/50 hover:bg-slate-100 dark:hover:bg-slate-800"
                      }`}
                    >
                      <span className="font-mono text-sm text-slate-700 dark:text-slate-200">
                        {new Date(snap.timestamp).toLocaleString()}
                      </span>
                      {isSelected && (
                        <span className="px-2 py-0.5 text-xs bg-accent-500 text-white rounded">
                          {selectionIndex === 0 ? "A (Before)" : "B (After)"}
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </>
      )}

      {mode === "diff" && diff && (
        <>
          <div className="flex items-center justify-between mb-4">
            <div className="text-sm">
              <span className="text-slate-500 dark:text-slate-400">
                Comparing:{" "}
              </span>
              <span className="font-mono text-accent-600 dark:text-accent-400">
                {new Date(diff.before).toLocaleString()}
              </span>
              <span className="text-slate-500 mx-2">&rarr;</span>
              <span className="font-mono text-accent-600 dark:text-accent-400">
                {new Date(diff.after).toLocaleString()}
              </span>
            </div>
            <button
              onClick={() => {
                setMode("snapshots");
                setDiff(null);
              }}
              className="px-3 py-1.5 text-sm bg-slate-200 dark:bg-slate-700 hover:bg-slate-300 dark:hover:bg-slate-600 text-slate-700 dark:text-slate-200 rounded-lg transition-colors"
            >
              Back to Snapshots
            </button>
          </div>

          <div className="grid grid-cols-3 gap-4 mb-4">
            <div className="bg-emerald-500/10 border border-emerald-500/30 rounded-lg p-4">
              <div className="text-2xl font-bold text-emerald-600 dark:text-emerald-400">
                {diff.added.length}
              </div>
              <div className="text-sm text-slate-500 dark:text-slate-400">
                Added
              </div>
            </div>
            <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-4">
              <div className="text-2xl font-bold text-red-600 dark:text-red-400">
                {diff.removed.length}
              </div>
              <div className="text-sm text-slate-500 dark:text-slate-400">
                Removed
              </div>
            </div>
            <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-4">
              <div className="text-2xl font-bold text-yellow-600 dark:text-yellow-400">
                {diff.modified.length}
              </div>
              <div className="text-sm text-slate-500 dark:text-slate-400">
                Modified
              </div>
            </div>
          </div>

          <div className="flex-1 bg-white dark:bg-slate-800/50 rounded-xl border border-slate-200 dark:border-slate-700/50 overflow-auto p-4 space-y-4 transition-colors">
            {diff.added.length > 0 && (
              <div>
                <h4 className="text-sm font-medium text-emerald-600 dark:text-emerald-400 mb-2">
                  + Added
                </h4>
                <div className="space-y-1">
                  {diff.added.slice(0, 50).map((r, i) => (
                    <div
                      key={i}
                      className="text-sm font-mono text-slate-700 dark:text-slate-300"
                    >
                      {r.kind} {r.namespace}/{r.name}
                    </div>
                  ))}
                  {diff.added.length > 50 && (
                    <div className="text-slate-500 text-sm">
                      ...and {diff.added.length - 50} more
                    </div>
                  )}
                </div>
              </div>
            )}
            {diff.removed.length > 0 && (
              <div>
                <h4 className="text-sm font-medium text-red-600 dark:text-red-400 mb-2">
                  - Removed
                </h4>
                <div className="space-y-1">
                  {diff.removed.slice(0, 50).map((r, i) => (
                    <div
                      key={i}
                      className="text-sm font-mono text-slate-700 dark:text-slate-300"
                    >
                      {r.kind} {r.namespace}/{r.name}
                    </div>
                  ))}
                  {diff.removed.length > 50 && (
                    <div className="text-slate-500 text-sm">
                      ...and {diff.removed.length - 50} more
                    </div>
                  )}
                </div>
              </div>
            )}
            {diff.modified.length > 0 && (
              <div>
                <h4 className="text-sm font-medium text-yellow-600 dark:text-yellow-400 mb-2">
                  ~ Modified
                </h4>
                <div className="space-y-1">
                  {diff.modified.slice(0, 50).map((r, i) => (
                    <div
                      key={i}
                      className="text-sm font-mono text-slate-700 dark:text-slate-300"
                    >
                      {r.kind} {r.namespace}/{r.name}
                      {r.oldStatus && r.newStatus && (
                        <span className="text-slate-500 ml-2">
                          ({r.oldStatus} &rarr; {r.newStatus})
                        </span>
                      )}
                    </div>
                  ))}
                  {diff.modified.length > 50 && (
                    <div className="text-slate-500 text-sm">
                      ...and {diff.modified.length - 50} more
                    </div>
                  )}
                </div>
              </div>
            )}
            {diff.added.length === 0 &&
              diff.removed.length === 0 &&
              diff.modified.length === 0 && (
                <p className="text-slate-400 text-center py-8">
                  No changes between these snapshots
                </p>
              )}
          </div>
        </>
      )}
    </div>
  );
}
