import { useEffect, useRef, useMemo, useState } from 'react';
import * as d3 from 'd3';
import { Case } from '../workspace/types';
import { getASRProviderConfig, isProviderEnabled } from '../config';
import { computeWeightedKDE } from '../utils/statistics';
import { X } from 'lucide-react';


interface StatsDashboardProps {
  cases: Case[];
  isOpen: boolean;
  onClose: () => void;
}

type MetricType = 'Q' | 'S' | 'P';

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
  // Raw data for distributions
  qData: { score: number; tokens: number }[];
  sData: { score: number; tokens: number }[];
  pData: { score: number; tokens: number }[];
}


export function StatsDashboard({ cases, isOpen, onClose }: StatsDashboardProps) {
  const d3Container = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState(0);
  const [activeMetric, setActiveMetric] = useState<MetricType>('Q');


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
            qData: [],
            sData: [],
            pData: [],
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

        // Push raw scores (S and P normalized to 0-100 for consistent distribution visualization)
        s.qData.push({ score: metrics.Q_score, tokens: tokenCount });
        s.sData.push({ score: metrics.S_score * 100, tokens: tokenCount });
        s.pData.push({ score: metrics.P_score * 100, tokens: tokenCount });
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

    // Sort by the active metric
    return result.sort((a, b) => {
      if (activeMetric === 'Q') return b.weightedQ - a.weightedQ;
      if (activeMetric === 'S') return b.weightedS - a.weightedS;
      return b.weightedP - a.weightedP;
    });
  }, [cases, activeMetric]);

  useEffect(() => {
    if (!d3Container.current || stats.length === 0 || !isOpen) return;

    const svg = d3.select(d3Container.current);
    svg.selectAll("*").remove();

    // Remove any stale tooltips
    d3.select(".stats-tooltip").remove();

    // Create tooltip div
    const tooltip = d3.select("body").append("div")
      .attr("class", "stats-tooltip fixed z-[100] bg-white dark:bg-slate-800 shadow-2xl rounded-xl border border-slate-200 dark:border-slate-700 p-3 text-[11px] pointer-events-none opacity-0 transition-opacity duration-200")
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

    // X Axis: Score 0-100
    const x = d3.scaleLinear().domain([0, 100]).range([0, innerWidth]);

    g.append("g")
      .attr("transform", `translate(0,${innerHeight})`)
      .call(d3.axisBottom(x).ticks(10))
      .selectAll("text")
      .attr("class", "text-xs fill-slate-500 dark:fill-slate-400");

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
      .attr("class", "stroke-slate-200 dark:stroke-slate-700")
      .attr("stroke-dasharray", "2,3");

    // Calculate density data based on active metric
    const densityData = stats.map(s => {
      // Select appropriate data based on activeMetric
      let rawData;
      if (activeMetric === 'S') rawData = s.sData;
      else if (activeMetric === 'P') rawData = s.pData;
      else rawData = s.qData;

      // Prepare data for KDE: { value: score, weight: tokens }
      const data = rawData.map(d => ({ value: d.score, weight: d.tokens }));

      // Compute KDE
      const density = computeWeightedKDE(data, [0, 100], 101, 2);
      const maxDensity = d3.max(density, (d: [number, number]) => d[1]) || 0;
      return { ...s, density, maxDensity };
    });

    // Global max weighted density for proportional scaling
    const globalMaxDensity = d3.max(densityData, d => d.maxDensity) || 1;

    // Draw each provider row
    densityData.forEach(d => {
      const bandWidth = y.bandwidth();
      const maxViolinHeight = bandWidth * 0.95;
      const yNum = d3.scaleLinear().domain([0, globalMaxDensity]).range([0, maxViolinHeight]);
      // Baseline aligned with the middle of the band
      const baselineY = (y(d.provider) || 0) + bandWidth / 2;

      // Group for this provider's elements
      const providerGroup = g.append("g").attr("class", `provider-${d.provider}`);

      // Draw top-half violin
      const area = d3.area<[number, number]>()
        .x(pt => x(pt[0]))
        .y0(() => baselineY)
        .y1(pt => baselineY - yNum(pt[1]))
        .curve(d3.curveMonotoneX);

      const violinPath = providerGroup.append("path")
        .datum(d.density)
        .attr("d", area)
        .attr("class", d.fillClass)
        .attr("fill-opacity", 0.3);

      // Dotted baseline guide
      providerGroup.append("line")
        .attr("x1", 0)
        .attr("x2", innerWidth)
        .attr("y1", baselineY)
        .attr("y2", baselineY)
        .attr("class", "stroke-slate-300 dark:stroke-slate-600")
        .attr("stroke-dasharray", "4,4")
        .attr("stroke-width", 1);

      // Center Dot (Weighted Score for Active Metric)
      let activeWeightedScore = d.weightedQ;
      if (activeMetric === 'S') activeWeightedScore = d.weightedS;
      if (activeMetric === 'P') activeWeightedScore = d.weightedP;

      providerGroup.append("foreignObject")
        .attr("x", x(activeWeightedScore) - 6)
        .attr("y", baselineY - 6)
        .attr("width", 12)
        .attr("height", 12)
        .append("xhtml:div")
        .attr("class", `w-3 h-3 rounded-full ${d.dotClass} border-2 border-white dark:border-slate-800 shadow-sm`);

      // Invisible hover rect
      providerGroup.append("rect")
        .attr("x", 0)
        .attr("y", y(d.provider) || 0)
        .attr("width", innerWidth)
        .attr("height", bandWidth)
        .attr("fill", "transparent")
        .attr("cursor", "pointer")
        .on("mouseenter", function (event: MouseEvent) {
          violinPath.attr("fill-opacity", 0.55).attr("stroke-width", 2.5);
          tooltip.style("opacity", 1).html(
            `<div class="font-bold mb-1 flex items-center gap-1.5 text-slate-900 dark:text-slate-100"><span class="w-2.5 h-2.5 rounded-full ${d.dotClass} inline-block"></span>${d.displayName}</div>` +
            `<div class="grid grid-cols-[auto_1fr] gap-x-2 gap-y-0.5 text-slate-600 dark:text-slate-400">` +
            `<span class="text-slate-400 ${activeMetric === 'Q' ? 'font-bold' : ''}">W-Q</span><span class="text-right ${activeMetric === 'Q' ? 'font-bold' : ''}">${d.weightedQ.toFixed(1)}</span>` +
            `<span class="text-slate-400 ${activeMetric === 'S' ? 'font-bold' : ''}">W-S</span><span class="text-right ${activeMetric === 'S' ? 'font-bold' : ''}">${d.weightedS.toFixed(1)}</span>` +
            `<span class="text-slate-400 ${activeMetric === 'P' ? 'font-bold' : ''}">W-P</span><span class="text-right ${activeMetric === 'P' ? 'font-bold' : ''}">${d.weightedP.toFixed(1)}</span>` +
            `<span class="text-slate-400">Cases</span><span class="text-right">${d.caseCount}</span>` +
            `<span class="text-slate-400">Tokens</span><span class="text-right">${d.totalTokens.toLocaleString()}</span>` +
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

    // Draw Provider Labels
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
        .attr("class", "text-slate-700 dark:text-slate-200")
        .style("line-height", "1.2")
        .style("overflow", "hidden")
        .style("text-overflow", "ellipsis")
        .style("padding-right", "8px")
        .text(d.displayName);
    });

    return () => { tooltip.remove(); };
  }, [stats, isOpen, containerWidth, activeMetric]);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/30 flex items-center justify-center z-50 backdrop-blur-sm" onClick={onClose}>
      <div
        className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl shadow-2xl w-[85vw] max-w-[1100px] max-h-[85vh] flex flex-col overflow-hidden"
        onClick={e => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-2.5 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shrink-0">
          <span className="text-sm font-bold text-slate-700 dark:text-slate-200">Performance Statistics</span>
          <button onClick={onClose} className="p-1.5 hover:bg-slate-100 dark:hover:bg-slate-800 rounded text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors">
            <X size={16} />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-y-auto flex flex-col">
          <div className="px-6 py-4" ref={containerRef}>
            <svg ref={d3Container} className="w-full" />
          </div>

          <div className="px-6 pb-6 pt-2 flex flex-col gap-6">
            {/* Controls */}
            <div className="flex justify-center">
              <div className="inline-flex bg-slate-100 dark:bg-slate-800 p-1 rounded-lg">
                {(['Q', 'S', 'P'] as MetricType[]).map(metric => (
                  <button
                    key={metric}
                    onClick={() => setActiveMetric(metric)}
                    className={`px-4 py-1.5 text-xs font-semibold rounded-md transition-all ${activeMetric === metric
                      ? 'bg-white dark:bg-slate-600 text-slate-800 dark:text-slate-100 shadow-sm'
                      : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200'
                      }`}
                  >
                    W-{metric} Score
                  </button>
                ))}
              </div>
            </div>

            {/* Table */}
            <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden">
              <table className="w-full text-xs text-left">
                <thead className="bg-slate-50 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 font-semibold border-b border-slate-200 dark:border-slate-800">
                  <tr>
                    <th className="px-4 py-3">Provider</th>
                    <th className={`px-4 py-3 text-right ${activeMetric === 'Q' ? 'font-bold text-slate-800 dark:text-slate-100' : ''}`}>W-Q Score</th>
                    <th className={`px-4 py-3 text-right ${activeMetric === 'S' ? 'font-bold text-slate-800 dark:text-slate-100' : ''}`}>W-S Score</th>
                    <th className={`px-4 py-3 text-right ${activeMetric === 'P' ? 'font-bold text-slate-800 dark:text-slate-100' : ''}`}>W-P Score</th>
                    <th className="px-4 py-3 text-right">Cases</th>
                    <th className="px-4 py-3 text-right">Tokens</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-800/50">
                  {stats.map(s => (
                    <tr key={s.provider} className="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
                      <td className="px-4 py-2.5 font-medium text-slate-700 dark:text-slate-200 flex items-center gap-2">
                        <span className={`w-2 h-2 rounded-full ${s.dotClass}`}></span>
                        {s.displayName}
                      </td>
                      <td className={`px-4 py-2.5 text-right font-mono ${activeMetric === 'Q' ? 'font-bold text-slate-800 dark:text-slate-100' : 'text-slate-600 dark:text-slate-400'}`}>
                        {s.weightedQ.toFixed(1)}
                      </td>
                      <td className={`px-4 py-2.5 text-right font-mono ${activeMetric === 'S' ? 'font-bold text-slate-800 dark:text-slate-100' : 'text-slate-600 dark:text-slate-400'}`}>
                        {s.weightedS.toFixed(1)}
                      </td>
                      <td className={`px-4 py-2.5 text-right font-mono ${activeMetric === 'P' ? 'font-bold text-slate-800 dark:text-slate-100' : 'text-slate-600 dark:text-slate-400'}`}>
                        {s.weightedP.toFixed(1)}
                      </td>
                      <td className="px-4 py-2.5 text-right text-slate-500 dark:text-slate-400">
                        {s.caseCount}
                      </td>
                      <td className="px-4 py-2.5 text-right text-slate-500 dark:text-slate-400">
                        {s.totalTokens.toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
