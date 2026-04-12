import { useState, useEffect } from "react";
import {
  ShieldCheck,
  ChevronDown,
  ChevronRight,
  RefreshCw,
  Loader2,
  AlertTriangle,
  User,
  Users,
  Bot,
} from "lucide-react";
import { GetNamespaceRBAC } from "../../../wailsjs/go/main/App";

interface PolicyRule {
  verbs: string[];
  resources: string[];
  apiGroups: string[];
  isWildcard: boolean;
}

interface RBACBinding {
  name: string;
  kind: string;
  roleName: string;
  roleKind: string;
  namespace: string;
  clusterWide: boolean;
}

interface SubjectPermissions {
  subject: string;
  kind: string;
  bindings: RBACBinding[];
  rules: PolicyRule[];
}

interface RBACMatrix {
  namespace: string;
  subjects: SubjectPermissions[];
  warning?: string;
}

function subjectIcon(kind: string) {
  if (kind === "ServiceAccount") return <Bot className="w-3.5 h-3.5 text-blue-400" />;
  if (kind === "Group") return <Users className="w-3.5 h-3.5 text-purple-400" />;
  return <User className="w-3.5 h-3.5 text-green-400" />;
}

function VerbPill({ verb, isWildcard }: { verb: string; isWildcard: boolean }) {
  const color = isWildcard || verb === "*"
    ? "bg-red-500/20 text-red-400 border-red-500/30"
    : verb === "get" || verb === "list" || verb === "watch"
    ? "bg-blue-500/20 text-blue-300 border-blue-500/30"
    : "bg-amber-500/20 text-amber-300 border-amber-500/30";
  return (
    <span className={`text-[10px] font-mono px-1.5 py-0.5 rounded border ${color}`}>
      {verb}
    </span>
  );
}

function SubjectRow({ sp }: { sp: SubjectPermissions }) {
  const [expanded, setExpanded] = useState(false);
  const isWildcard = sp.rules.some((r) => r.isWildcard);

  return (
    <>
      <tr
        className="border-b border-stone-100 dark:border-slate-700/50 hover:bg-stone-50 dark:hover:bg-slate-700/30 cursor-pointer"
        onClick={() => setExpanded((v) => !v)}
      >
        <td className="px-3 py-2">
          {expanded
            ? <ChevronDown className="w-3.5 h-3.5 text-stone-400" />
            : <ChevronRight className="w-3.5 h-3.5 text-stone-400" />}
        </td>
        <td className="px-3 py-2">
          <div className="flex items-center gap-1.5">
            {subjectIcon(sp.kind)}
            <span className="text-sm font-mono text-stone-800 dark:text-slate-200">{sp.subject}</span>
          </div>
        </td>
        <td className="px-3 py-2">
          <span className="text-xs text-stone-500 dark:text-slate-400">{sp.kind}</span>
        </td>
        <td className="px-3 py-2">
          <span className="text-xs text-stone-500 dark:text-slate-400">{sp.bindings.length}</span>
        </td>
        <td className="px-3 py-2">
          <div className="flex flex-wrap gap-1 max-w-xs">
            {isWildcard ? (
              <span className="text-[10px] font-mono px-1.5 py-0.5 rounded border bg-red-500/20 text-red-400 border-red-500/30">
                cluster-admin / *
              </span>
            ) : (
              [...new Set(sp.rules.flatMap((r) => r.verbs))].slice(0, 6).map((v) => (
                <VerbPill key={v} verb={v} isWildcard={false} />
              ))
            )}
          </div>
        </td>
        <td className="px-3 py-2">
          <div className="flex flex-wrap gap-1 max-w-xs">
            {isWildcard ? (
              <span className="text-[10px] font-mono text-red-400">all resources</span>
            ) : (
              [...new Set(sp.rules.flatMap((r) => r.resources))].slice(0, 5).map((res) => (
                <span key={res} className="text-[10px] font-mono text-stone-500 dark:text-slate-400">
                  {res}
                </span>
              ))
            )}
          </div>
        </td>
      </tr>
      {expanded && (
        <tr className="bg-stone-50/50 dark:bg-slate-800/30">
          <td colSpan={6} className="px-6 py-3">
            <div className="space-y-3">
              <div>
                <h4 className="text-xs font-medium text-stone-500 dark:text-slate-400 mb-1">Bindings</h4>
                <div className="flex flex-wrap gap-2">
                  {sp.bindings.map((b, i) => (
                    <div key={i} className="flex items-center gap-1 text-xs bg-white dark:bg-slate-900 border border-stone-200 dark:border-slate-700 rounded px-2 py-1">
                      <span className="font-mono text-stone-700 dark:text-slate-300">{b.roleName}</span>
                      <span className="text-stone-400">via</span>
                      <span className="text-stone-500 dark:text-slate-400">{b.kind}</span>
                      {b.clusterWide && <span className="text-[10px] bg-red-500/10 text-red-400 px-1 rounded">cluster-wide</span>}
                    </div>
                  ))}
                </div>
              </div>
              <div>
                <h4 className="text-xs font-medium text-stone-500 dark:text-slate-400 mb-1">Rules</h4>
                <div className="space-y-1">
                  {sp.rules.map((rule, i) => (
                    <div key={i} className="flex items-start gap-3 text-xs">
                      <div className="flex flex-wrap gap-1 min-w-[140px]">
                        {rule.verbs.map((v) => <VerbPill key={v} verb={v} isWildcard={rule.isWildcard} />)}
                      </div>
                      <span className="text-stone-400 dark:text-slate-500">on</span>
                      <span className="font-mono text-stone-600 dark:text-slate-300">{rule.resources.join(", ")}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  );
}

export function RBACView({
  isConnected,
  namespaces,
  activeContext,
}: {
  isConnected: boolean;
  namespaces: string[];
  activeContext: string;
}) {
  const [namespace, setNamespace] = useState(namespaces[0] ?? "default");
  const [matrix, setMatrix] = useState<RBACMatrix | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    if (!isConnected) return;
    setLoading(true);
    setError(null);
    try {
      const m = await GetNamespaceRBAC(activeContext, namespace);
      setMatrix(m as RBACMatrix);
    } catch (e: any) {
      setError(e?.message ?? String(e));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [namespace, activeContext, isConnected]);

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center gap-3 mb-4">
        <ShieldCheck className="w-5 h-5 text-green-400" />
        <h2 className="text-lg font-semibold text-stone-800 dark:text-slate-100">RBAC Viewer</h2>
        <select
          value={namespace}
          onChange={(e) => setNamespace(e.target.value)}
          className="ml-auto text-sm bg-white dark:bg-slate-800 border border-stone-200 dark:border-slate-700 rounded-lg px-3 py-1.5 text-stone-700 dark:text-slate-200"
        >
          {namespaces.map((ns) => <option key={ns} value={ns}>{ns}</option>)}
        </select>
        <button
          onClick={load}
          disabled={loading}
          className="p-2 rounded-lg hover:bg-stone-100 dark:hover:bg-slate-700 text-stone-500 dark:text-slate-400 transition-colors"
        >
          {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <RefreshCw className="w-4 h-4" />}
        </button>
      </div>

      {/* Warning */}
      {matrix?.warning && (
        <div className="flex items-center gap-2 mb-3 px-3 py-2 bg-yellow-500/10 border border-yellow-500/30 rounded-lg text-sm text-yellow-600 dark:text-yellow-400">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          {matrix.warning}
        </div>
      )}

      {/* Error */}
      {error && (
        <div className="flex items-center gap-2 mb-3 px-3 py-2 bg-red-500/10 border border-red-500/30 rounded-lg text-sm text-red-600 dark:text-red-400">
          <AlertTriangle className="w-4 h-4 flex-shrink-0" />
          {error}
        </div>
      )}

      {/* Table */}
      <div className="flex-1 overflow-auto rounded-lg border border-stone-200 dark:border-slate-700">
        <table className="w-full text-left text-sm">
          <thead className="sticky top-0 bg-stone-100 dark:bg-slate-800 border-b border-stone-200 dark:border-slate-700">
            <tr>
              <th className="px-3 py-2 w-6" />
              <th className="px-3 py-2 text-xs font-medium text-stone-600 dark:text-slate-400 uppercase tracking-wide">Subject</th>
              <th className="px-3 py-2 text-xs font-medium text-stone-600 dark:text-slate-400 uppercase tracking-wide">Kind</th>
              <th className="px-3 py-2 text-xs font-medium text-stone-600 dark:text-slate-400 uppercase tracking-wide">Bindings</th>
              <th className="px-3 py-2 text-xs font-medium text-stone-600 dark:text-slate-400 uppercase tracking-wide">Verbs</th>
              <th className="px-3 py-2 text-xs font-medium text-stone-600 dark:text-slate-400 uppercase tracking-wide">Resources</th>
            </tr>
          </thead>
          <tbody>
            {loading && !matrix && (
              <tr>
                <td colSpan={6} className="text-center py-12 text-stone-400">
                  <Loader2 className="w-5 h-5 animate-spin mx-auto mb-2" />
                  Loading RBAC data…
                </td>
              </tr>
            )}
            {!loading && matrix?.subjects?.length === 0 && (
              <tr>
                <td colSpan={6} className="text-center py-12 text-stone-400 dark:text-slate-500">
                  No bindings found in namespace <strong>{namespace}</strong>
                </td>
              </tr>
            )}
            {matrix?.subjects?.map((sp, i) => <SubjectRow key={i} sp={sp} />)}
          </tbody>
        </table>
      </div>

      <p className="mt-2 text-xs text-stone-400 dark:text-slate-500">
        Read-only view. Shows RoleBindings and ClusterRoleBindings affecting this namespace.
      </p>
    </div>
  );
}
