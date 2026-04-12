import { memo } from "react";
import type { Position } from "@xyflow/react";
import { BaseEdge, getSmoothStepPath } from "@xyflow/react";
import { useTheme } from "next-themes";
import { motion } from "framer-motion";

interface AnimatedEdgeProps {
  id: string;
  sourceX: number;
  sourceY: number;
  targetX: number;
  targetY: number;
  sourcePosition: Position;
  targetPosition: Position;
  style?: React.CSSProperties;
  data?: {
    edgeType?: string;
    isHighlighted?: boolean;
    isUpstream?: boolean;
  };
}

function AnimatedEdgeComponent({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  style = {},
  data,
}: AnimatedEdgeProps) {
  const { resolvedTheme } = useTheme();
  const [edgePath] = getSmoothStepPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  const getEdgeColor = () => {
    if (data?.isHighlighted) {
      return data?.isUpstream ? "#22d3ee" : "#fb923c"; // cyan for upstream, orange for downstream
    }
    switch (data?.edgeType) {
      case "service-to-pod":
        return resolvedTheme === "dark" ? "#06b6d4" : "#0891b2"; // cyan-500 : cyan-600
      case "ingress-to-service":
        return resolvedTheme === "dark" ? "#a855f7" : "#9333ea"; // purple-500 : purple-600
      case "controller-to-pod":
        return resolvedTheme === "dark" ? "#22c55e" : "#16a34a"; // green-500 : green-600
      case "node-to-pod":
        return resolvedTheme === "dark" ? "#6366f1" : "#4f46e5"; // indigo-500 : indigo-600
      case "operator-to-managed":
        return resolvedTheme === "dark" ? "#8b5cf6" : "#7c3aed"; // violet-500 : violet-600
      default:
        return resolvedTheme === "dark" ? "#64748b" : "#475569"; // slate-500 : slate-600
    }
  };

  const strokeWidth = data?.isHighlighted ? 3 : 1.5;
  const opacity = data?.isHighlighted === false ? 0.2 : 1;

  return (
    <>
      <BaseEdge
        id={id}
        path={edgePath}
        style={{
          ...style,
          stroke: getEdgeColor(),
          strokeWidth,
          opacity,
        }}
      />
      {data?.isHighlighted && (
        <motion.circle
          r={4}
          fill={getEdgeColor()}
          initial={{ offsetDistance: "0%" }}
          animate={{ offsetDistance: "100%" }}
          transition={{
            duration: 1.5,
            repeat: Infinity,
            ease: "linear",
          }}
          style={{
            offsetPath: `path("${edgePath}")`,
          }}
        />
      )}
    </>
  );
}

export const AnimatedEdge = memo(AnimatedEdgeComponent);
