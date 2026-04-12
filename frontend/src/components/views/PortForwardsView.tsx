import { useState, useEffect, useRef } from "react";
import { ArrowUpDown, ChevronUp, ChevronDown } from "lucide-react";
import {
  ListPortForwards,
  CreatePortForward,
  StopPortForward,
  ListResources,
} from "../../../wailsjs/go/main/App";
import type { ResourceInfo, PortForwardInfo } from "../../types/resources";

export function PortForwardsView({ isConnected }: { isConnected: boolean }) {
  const [forwards, setForwards] = useState<PortForwardInfo[]>([]);
  const [pods, setPods] = useState<ResourceInfo[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [selectedPod, setSelectedPod] = useState<string>("");
  const [selectedNamespace, setSelectedNamespace] = useState<string>("");
  const [localPort, setLocalPort] = useState<string>("");
  const [remotePort, setRemotePort] = useState<string>("");
  const [creating, setCreating] = useState(false);
  const [namespaceFilter, setNamespaceFilter] = useState("");
  const [selectedIndex, setSelectedIndex] = useState<number>(-1);
  const tableRef = useRef<HTMLDivElement>(null);

  // Sorting
  const [sortField, setSortField] = useState<
    "pod" | "local" | "remote" | "status"
  >("pod");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");

  const handleSort = (field: "pod" | "local" | "remote" | "status") => {
    if (sortField === field) {
      setSortDirection((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDirection("asc");
    }
  };

  const SortIndicator = ({ field }: { field: string }) => {
    if (sortField !== field) {
      return <ArrowUpDown size={14} className="text-slate-600" />;
    }
    return sortDirection === "asc" ? (
      <ChevronUp size={14} className="text-accent-400" />
    ) : (
      <ChevronDown size={14} className="text-accent-400" />
    );
  };

  const fetchForwards = async () => {
    try {
      const result = await ListPortForwards();
      setForwards(result || []);
    } catch (err) {
      console.error("Failed to fetch port forwards:", err);
    }
  };

  const fetchPods = async () => {
    try {
      const result = await ListResources("pods", namespaceFilter);
      setPods(result || []);
    } catch (err) {
      console.error("Failed to fetch pods:", err);
    }
  };

  useEffect(() => {
    if (isConnected) {
      fetchForwards();
      fetchPods();
      const interval = setInterval(fetchForwards, 2000);
      return () => clearInterval(interval);
    } else {
      setForwards([]);
      setPods([]);
    }
  }, [isConnected, namespaceFilter]);

  // Reset selection when forwards change
  useEffect(() => {
    setSelectedIndex(-1);
  }, [forwards.length]);

  // Keyboard navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (
        target.tagName === "INPUT" ||
        target.tagName === "TEXTAREA" ||
        target.tagName === "SELECT" ||
        target.isContentEditable
      ) {
        return;
      }
      switch (e.key) {
        case "n":
          e.preventDefault();
          setShowCreateForm((prev) => !prev);
          break;
        case "Escape":
          if (showCreateForm) {
            e.preventDefault();
            setShowCreateForm(false);
          }
          break;
        case "ArrowDown":
        case "j":
          if (showCreateForm) return;
          e.preventDefault();
          setSelectedIndex((prev) =>
            prev < forwards.length - 1 ? prev + 1 : prev
          );
          break;
        case "ArrowUp":
        case "k":
          if (showCreateForm) return;
          e.preventDefault();
          setSelectedIndex((prev) => (prev > 0 ? prev - 1 : 0));
          break;
      }

      // Shift+letter sorting shortcuts
      if (e.shiftKey && !showCreateForm) {
        switch (e.key) {
          case "P":
            e.preventDefault();
            handleSort("pod");
            break;
          case "L":
            e.preventDefault();
            handleSort("local");
            break;
          case "R":
            e.preventDefault();
            handleSort("remote");
            break;
          case "S":
            e.preventDefault();
            handleSort("status");
            break;
        }
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [forwards.length, showCreateForm, sortField]);

  // Scroll selected row into view
  useEffect(() => {
    if (selectedIndex >= 0 && tableRef.current) {
      const row = tableRef.current.querySelector(
        `tr[data-index="${selectedIndex}"]`
      );
      row?.scrollIntoView({ block: "nearest", behavior: "smooth" });
    }
  }, [selectedIndex]);

  const handleCreate = async () => {
    if (!selectedPod || !selectedNamespace || !localPort || !remotePort) return;
    setCreating(true);
    setError(null);
    try {
      await CreatePortForward(
        selectedNamespace,
        selectedPod,
        parseInt(localPort),
        parseInt(remotePort)
      );
      setShowCreateForm(false);
      setSelectedPod("");
      setSelectedNamespace("");
      setLocalPort("");
      setRemotePort("");
      await fetchForwards();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to create port forward"
      );
    } finally {
      setCreating(false);
    }
  };

  const handleStop = async (id: string) => {
    try {
      await StopPortForward(id);
      await fetchForwards();
    } catch (err) {
      console.error("Failed to stop port forward:", err);
    }
  };

  const getStatusColor = (status: string) => {
    if (status === "active") return "text-emerald-400";
    if (status === "starting") return "text-yellow-400";
    return "text-red-400";
  };

  return (
    <div className="h-full flex flex-col">
      <div className="flex items-center justify-between mb-4">
        <div className="flex gap-3 items-center">
          <input
            type="text"
            placeholder="Filter pods by namespace..."
            value={namespaceFilter}
            onChange={(e) => setNamespaceFilter(e.target.value)}
            className="w-56 bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50 text-stone-800 dark:text-slate-100 placeholder-stone-400 dark:placeholder-slate-500"
          />
        </div>
        <button
          onClick={() => setShowCreateForm(!showCreateForm)}
          disabled={!isConnected}
          className="px-4 py-2 text-sm bg-accent-500 hover:bg-accent-600 text-white rounded-lg transition-colors disabled:opacity-50"
        >
          {showCreateForm ? "Cancel" : "New Port Forward"}
        </button>
      </div>

      {showCreateForm && (
        <div className="bg-white dark:bg-slate-800/50 rounded-xl border border-stone-200 dark:border-slate-700/50 p-4 mb-4 shadow-sm dark:shadow-none transition-colors">
          <h3 className="text-sm font-medium mb-3 text-stone-800 dark:text-slate-100">
            Create Port Forward
          </h3>
          {error && <p className="text-red-400 text-sm mb-3">{error}</p>}
          <div className="grid grid-cols-2 gap-4 mb-4">
            <div>
              <label className="block text-xs text-stone-400 dark:text-slate-400 mb-1">
                Pod
              </label>
              <select
                value={`${selectedNamespace}/${selectedPod}`}
                onChange={(e) => {
                  const [ns, ...rest] = e.target.value.split("/");
                  setSelectedNamespace(ns);
                  setSelectedPod(rest.join("/"));
                }}
                className="bg-white dark:bg-slate-900 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50 text-stone-800 dark:text-slate-200 transition-colors duration-200"
              >
                <option value="/">Select a pod...</option>
                {pods
                  .filter((p) => p.status === "Running")
                  .map((pod) => (
                    <option
                      key={`${pod.namespace}/${pod.name}`}
                      value={`${pod.namespace}/${pod.name}`}
                    >
                      {pod.namespace}/{pod.name}
                    </option>
                  ))}
              </select>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-xs text-stone-400 dark:text-slate-400 mb-1">
                  Local Port
                </label>
                <input
                  type="number"
                  value={localPort}
                  onChange={(e) => setLocalPort(e.target.value)}
                  placeholder="8080"
                  className="bg-white dark:bg-slate-900 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50 text-stone-800 dark:text-slate-200 transition-colors duration-200"
                />
              </div>
              <div>
                <label className="block text-xs text-stone-400 dark:text-slate-400 mb-1">
                  Remote Port
                </label>
                <input
                  type="number"
                  value={remotePort}
                  onChange={(e) => setRemotePort(e.target.value)}
                  placeholder="80"
                  className="bg-white dark:bg-slate-900 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent-500/50 text-stone-800 dark:text-slate-200 transition-colors duration-200"
                />
              </div>
            </div>
          </div>
          <button
            onClick={handleCreate}
            disabled={
              creating ||
              !selectedPod ||
              !selectedNamespace ||
              !localPort ||
              !remotePort
            }
            className="px-4 py-2 text-sm bg-emerald-600 hover:bg-emerald-700 text-white rounded-lg transition-colors disabled:opacity-50 flex items-center gap-2"
          >
            {creating && (
              <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
            )}
            Create
          </button>
        </div>
      )}

      <div className="flex-1 bg-white dark:bg-slate-800/50 rounded-xl border border-stone-200 dark:border-slate-700/50 overflow-hidden shadow-sm dark:shadow-none transition-colors">
        {!isConnected ? (
          <p className="text-stone-400 dark:text-slate-400 text-center py-12">
            Connect to a cluster to manage port forwards
          </p>
        ) : forwards.length === 0 ? (
          <div className="text-center py-12">
            <p className="text-stone-400 dark:text-slate-400 mb-2">
              No active port forwards
            </p>
            <p className="text-sm text-stone-500 dark:text-slate-500">
              Click "New Port Forward" to create one
            </p>
          </div>
        ) : (
          <div ref={tableRef} className="overflow-auto h-full">
            <table className="w-full text-sm">
              <thead className="bg-stone-50 dark:bg-slate-900 sticky top-0 z-10 transition-colors">
                <tr className="text-left text-stone-500 dark:text-slate-400">
                  <th
                    className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none"
                    onClick={() => handleSort("pod")}
                  >
                    <span className="flex items-center gap-1">
                      Pod <SortIndicator field="pod" />
                    </span>
                  </th>
                  <th
                    className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none"
                    onClick={() => handleSort("local")}
                  >
                    <span className="flex items-center gap-1">
                      Local <SortIndicator field="local" />
                    </span>
                  </th>
                  <th
                    className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none"
                    onClick={() => handleSort("remote")}
                  >
                    <span className="flex items-center gap-1">
                      Remote <SortIndicator field="remote" />
                    </span>
                  </th>
                  <th
                    className="px-4 py-3 font-medium cursor-pointer hover:bg-stone-100 dark:hover:bg-slate-800 transition-colors select-none"
                    onClick={() => handleSort("status")}
                  >
                    <span className="flex items-center gap-1">
                      Status <SortIndicator field="status" />
                    </span>
                  </th>
                  <th className="px-4 py-3 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-200 dark:divide-slate-700/50">
                {[...forwards]
                  .sort((a, b) => {
                    let comparison = 0;
                    switch (sortField) {
                      case "pod":
                        comparison = `${a.namespace}/${a.pod}`.localeCompare(
                          `${b.namespace}/${b.pod}`
                        );
                        break;
                      case "local":
                        comparison = a.localPort - b.localPort;
                        break;
                      case "remote":
                        comparison = a.remotePort - b.remotePort;
                        break;
                      case "status":
                        comparison = a.status.localeCompare(b.status);
                        break;
                    }
                    return sortDirection === "asc" ? comparison : -comparison;
                  })
                  .map((fwd, idx) => (
                    <tr
                      key={fwd.id}
                      data-index={idx}
                      className={`transition-colors hover:bg-stone-50 dark:hover:bg-slate-700/30 ${
                        selectedIndex === idx
                          ? "bg-accent-500/20 ring-1 ring-accent-500/50"
                          : ""
                      }`}
                    >
                      <td className="px-4 py-3">
                        <span className="text-stone-500 dark:text-slate-500">
                          {fwd.namespace}/
                        </span>
                        <span className="font-mono text-accent-600 dark:text-accent-400">
                          {fwd.pod}
                        </span>
                      </td>
                      <td className="px-4 py-3 font-mono">
                        <a
                          href={`http://localhost:${fwd.localPort}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="text-cyan-400 hover:underline"
                        >
                          :{fwd.localPort}
                        </a>
                      </td>
                      <td className="px-4 py-3 font-mono text-stone-400 dark:text-slate-400">
                        :{fwd.remotePort}
                      </td>
                      <td className={`px-4 py-3 ${getStatusColor(fwd.status)}`}>
                        {fwd.status}
                        {fwd.error && (
                          <span className="text-red-400 text-xs ml-2">
                            ({fwd.error})
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <button
                          onClick={() => handleStop(fwd.id)}
                          className="px-2 py-1 text-xs bg-red-500/20 text-red-400 rounded hover:bg-red-500/30 transition-colors"
                        >
                          Stop
                        </button>
                      </td>
                    </tr>
                  ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <div className="mt-3 text-sm text-stone-500 dark:text-slate-500">
        {forwards.length} active port forward{forwards.length !== 1 ? "s" : ""}
      </div>
    </div>
  );
}
