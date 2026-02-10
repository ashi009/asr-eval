import { useEffect, useRef, useMemo, useState } from 'react';
import * as d3 from 'd3';
import { Case } from '../workspace/types';
import { getASRProviderConfig, isProviderEnabled } from '../config';
import { X } from 'lucide-react';


interface StatsDashboardProps {
  cases: Case[];
  isOpen: boolean;
  onClose: () => void;
}

interface ProviderStats {
  provider: string;
  displayName: string;
  fillClass: string;
  textClass: string;
  dotClass: string;
  weightedQ: number;
  weightedS: number;
  weightedP: number;
  totalTokens: number;
  caseCount: number;
  qScores: number[];
  scoreData: { score: number; tokens: number }[];
}

export function StatsDashboard({ cases, isOpen, onClose }: StatsDashboardProps) {
  const d3Container = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState(0);

  useEffect(() => {
    if (!containerRef.current) return;
    const observer = new ResizeObserver(entries => {
      for (let entry of entries) {
        setContainerWidth(entry.contentRect.width);
      }
    });
    observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, [isOpen]);

  const stats = useMemo(() => {
    const providerMap = new Map<string, ProviderStats>();

    cases.forEach(c => {
      if (!c.report_v2?.evaluations) return;
      const tokenCount = c.report_v2.context_snapshot?.meta?.total_token_count_estimate || 0;

      Object.entries(c.report_v2.evaluations).forEach(([provider, result]) => {
        if (!isProviderEnabled(provider)) return;
        const metrics = (result as any).metrics || (result as any).Metrics;
        if (!metrics) return;

        if (!providerMap.has(provider)) {
          const config = getASRProviderConfig(provider);
          providerMap.set(provider, {
            provider,
            displayName: config.name,
            fillClass: config.color.fill || 'fill-slate-400',
            textClass: config.color.text || 'text-slate-700',
            dotClass: config.color.dot || 'bg-slate-400',
            weightedQ: 0,
            weightedS: 0,
            weightedP: 0,
            totalTokens: 0,
            caseCount: 0,
            qScores: [],
            scoreData: [],
          });
        }

        const s = providerMap.get(provider)!;
        s.caseCount++;
        if (tokenCount > 0) {
          s.totalTokens += tokenCount;
          s.weightedQ += metrics.Q_score * tokenCount;
          s.weightedS += metrics.S_score * 100 * tokenCount;
          s.weightedP += metrics.P_score * 100 * tokenCount;
        }
        s.qScores.push(metrics.Q_score);
        s.scoreData.push({ score: metrics.Q_score, tokens: tokenCount });
      });
    });

    const result = Array.from(providerMap.values()).map(s => {
      if (s.totalTokens > 0) {
        s.weightedQ /= s.totalTokens;
        s.weightedS /= s.totalTokens;
        s.weightedP /= s.totalTokens;
      }
      return s;
    });

    return result.sort((a, b) => b.weightedQ - a.weightedQ);
  }, [cases]);

  useEffect(() => {
    if (!d3Container.current || stats.length === 0 || !isOpen) return;

    const svg = d3.select(d3Container.current);
    svg.selectAll("*").remove();

    // Remove any stale tooltips
    d3.select(".stats-tooltip").remove();

    // Create tooltip div
    // Create tooltip div matched to RichTooltip style
    const tooltip = d3.select("body").append("div")
      .attr("class", "stats-tooltip fixed z-[100] bg-white shadow-2xl rounded-xl border border-slate-200 p-3 text-[11px] pointer-events-none opacity-0 transition-opacity duration-200")
      .style("max-width", "220px");

    // Dynamic left margin based on longest label
    const maxLabelLen = d3.max(stats, d => d.displayName.length) || 10;
    const dynamicLeft = Math.min(Math.max(maxLabelLen * 6.5 + 12, 80), 180);
    const margin = { top: 20, right: 20, bottom: 24, left: dynamicLeft };
    const rowHeight = 56;
    const height = stats.length * rowHeight + margin.top + margin.bottom;
    const width = (containerWidth || containerRef.current?.clientWidth || 700);

    svg.attr("width", width).attr("height", height);

    const g = svg.append("g")
      .attr("transform", `translate(${margin.left},${margin.top})`);

    const innerWidth = width - margin.left - margin.right;
    const innerHeight = height - margin.top - margin.bottom;

    // X Axis: Q Score 0-100
    const x = d3.scaleLinear().domain([0, 100]).range([0, innerWidth]);

    g.append("g")
      .attr("transform", `translate(0,${innerHeight})`)
      .call(d3.axisBottom(x).ticks(10))
      .selectAll("text")
      .attr("class", "text-xs fill-slate-500");



    // Y Axis: Providers
    const y = d3.scaleBand()
      .domain(stats.map(d => d.provider))
      .range([0, innerHeight])
      .padding(0.15);



    // Grid lines
    g.append("g")
      .selectAll("line")
      .data(x.ticks(10))
      .enter()
      .append("line")
      .attr("x1", d => x(d))
      .attr("x2", d => x(d))
      .attr("y1", 0)
      .attr("y2", innerHeight)
      .attr("stroke", "#e2e8f0")
      .attr("stroke-dasharray", "2,3");

    // Exact density using binning (step 5) to smooth out spikes
    const densityData = stats.map(s => {
      // Initialize bins at 0, 5, 10... 100
      const binStep = 4;
      const bins = new Map<number, number>();
      for (let i = 0; i <= 100; i += binStep) {
        bins.set(i, 0);
      }

      s.scoreData.forEach(d => {
        // Snap to nearest 5
        const scoreIndex = Math.round(d.score / binStep) * binStep;
        if (bins.has(scoreIndex)) {
          bins.set(scoreIndex, bins.get(scoreIndex)! + d.tokens);
        }
      });

      // Map to [x, y] coordinates
      const density = Array.from(bins.entries())
        .sort((a, b) => a[0] - b[0])
        .map(([x, y]) => [x, y] as [number, number]);

      const maxDensity = d3.max(density, d => d[1]) || 0;
      return { ...s, density, maxDensity };
    });

    // Global max weighted density for proportional scaling
    const globalMaxDensity = d3.max(densityData, d => d.maxDensity) || 1;

    // Draw each provider row
    densityData.forEach(d => {
      const bandWidth = y.bandwidth();
      const maxViolinHeight = bandWidth * 0.95; // Use almost full height
      const yNum = d3.scaleLinear().domain([0, globalMaxDensity]).range([0, maxViolinHeight]);
      // Baseline aligned with the middle of the band (where text is)
      const baselineY = (y(d.provider) || 0) + bandWidth / 2;

      // Group for this provider's elements
      const providerGroup = g.append("g").attr("class", `provider-${d.provider}`);

      // Draw top-half violin (ridgeline: baseline at center, density grows upward)
      const area = d3.area<[number, number]>()
        .x(pt => x(pt[0]))
        .y0(() => baselineY)
        .y1(pt => baselineY - yNum(pt[1]))
        .curve(d3.curveMonotoneX); // Use monotone spline for smooth but accurate connections

      const violinPath = providerGroup.append("path")
        .datum(d.density)
        .attr("d", area)
        .attr("class", d.fillClass)
        .attr("fill-opacity", 0.3); // No stroke

      // Dotted baseline guide
      providerGroup.append("line")
        .attr("x1", 0)
        .attr("x2", innerWidth)
        .attr("y1", baselineY)
        .attr("y2", baselineY)
        .attr("stroke", "#cbd5e1")
        .attr("stroke-dasharray", "4,4")
        .attr("stroke-width", 1);

      // Center Dot (Weighted Q) - using styled foreignObject
      providerGroup.append("foreignObject")
        .attr("x", x(d.weightedQ) - 6)
        .attr("y", baselineY - 6)
        .attr("width", 12)
        .attr("height", 12)
        .append("xhtml:div")
        .attr("class", `w-3 h-3 rounded-full ${d.dotClass} border-2 border-white shadow-sm`);

      // Invisible hover rect covering the whole row
      providerGroup.append("rect")
        .attr("x", 0)
        .attr("y", y(d.provider) || 0)
        .attr("width", innerWidth)
        .attr("height", bandWidth)
        .attr("fill", "transparent")
        .attr("cursor", "pointer")
        .on("mouseenter", function (event: MouseEvent) {
          // Highlight
          violinPath.attr("fill-opacity", 0.55).attr("stroke-width", 2.5);
          // Show tooltip
          tooltip.style("opacity", 1).html(
            `<div style="font-weight:700;margin-bottom:4px;display:flex;align-items:center;gap:6px"><span class="w-2.5 h-2.5 rounded-full ${d.dotClass} inline-block"></span>${d.displayName}</div>` +
            `<div style="display:grid;grid-template-columns:auto 1fr;gap:2px 8px;color:#475569">` +
            `<span style="color:#94a3b8">W-Q</span><span style="font-weight:600;text-align:right">${d.weightedQ.toFixed(1)}</span>` +
            `<span style="color:#94a3b8">W-S</span><span style="text-align:right">${d.weightedS.toFixed(1)}</span>` +
            `<span style="color:#94a3b8">W-P</span><span style="text-align:right">${d.weightedP.toFixed(1)}</span>` +
            `<span style="color:#94a3b8">Cases</span><span style="text-align:right">${d.caseCount}</span>` +
            `<span style="color:#94a3b8">Tokens</span><span style="text-align:right">${d.totalTokens.toLocaleString()}</span>` +
            `</div>`
          );
          tooltip
            .style("left", `${event.clientX + 12}px`)
            .style("top", `${event.clientY - 10}px`);
        })
        .on("mousemove", function (event: MouseEvent) {
          tooltip
            .style("left", `${event.clientX + 12}px`)
            .style("top", `${event.clientY - 10}px`);
        })
        .on("mouseleave", function () {
          violinPath.attr("fill-opacity", 0.3).attr("stroke-width", 1.5);
          tooltip.style("opacity", 0);
        });
    });

    // Draw Provider Labels LAST so they are on top of everything (using foreignObject for wrapping)
    const labelGroup = g.append("g");
    stats.forEach(d => {
      const bandCenter = (y(d.provider) || 0) + y.bandwidth() / 2;
      const labelW = margin.left - 10;
      const labelH = y.bandwidth();
      labelGroup.append("foreignObject")
        .attr("x", -margin.left)
        .attr("y", bandCenter - labelH / 2)
        .attr("width", labelW)
        .attr("height", labelH)
        .append("xhtml:div")
        .style("width", "100%")
        .style("height", "100%")
        .style("display", "flex")
        .style("align-items", "center")
        .style("justify-content", "flex-end")
        .style("text-align", "right")
        .style("font-size", "11px")
        .style("font-weight", "600")
        .attr("class", d.textClass)
        .style("line-height", "1.2")
        .style("overflow", "hidden")
        .style("text-overflow", "ellipsis")
        .style("padding-right", "8px")
        .text(d.displayName);
    });

    // Cleanup tooltip on unmount
    return () => { tooltip.remove(); };
  }, [stats, isOpen, containerWidth]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm" onClick={onClose}>
      <div
        className="bg-white border border-slate-200 rounded-xl shadow-2xl w-[85vw] max-w-[1100px] max-h-[85vh] flex flex-col overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-2.5 border-b border-slate-200 bg-white shrink-0">
          <span className="text-sm font-bold text-slate-700">Performance Statistics</span>
          <button onClick={onClose} className="p-1.5 hover:bg-slate-100 rounded text-slate-400 hover:text-slate-600 transition-colors">
            <X size={16} />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto">
          <div className="px-6 py-4" ref={containerRef}>
            <svg ref={d3Container} className="w-full" />
          </div>
        </div>
      </div>
    </div>
  );
}
