/**
 * Computes a weighted Kernel Density Estimation (KDE).
 *
 * @param data Array of { value, weight } objects.
 * @param domain [min, max] range for the density estimation.
 * @param ticks Number of points to evaluate the density at (default 100).
 * @param bandwidth Optional bandwidth. If not provided, it's estimated using Silverman's Rule.
 * @returns Array of [x, density] tuples.
 */
export function computeWeightedKDE(
  data: { value: number; weight: number }[],
  domain: [number, number] = [0, 100],
  ticks: number = 100,
  bandwidth?: number
): [number, number][] {
  if (data.length === 0) return [];

  // 1. Calculate weighted mean and variance for bandwidth estimation
  let totalWeight = 0;
  let weightedSum = 0;
  let weightedSqSum = 0;

  for (const d of data) {
    totalWeight += d.weight;
    weightedSum += d.value * d.weight;
    weightedSqSum += d.value * d.value * d.weight;
  }

  if (totalWeight === 0) return [];

  const mean = weightedSum / totalWeight;
  const variance = (weightedSqSum / totalWeight) - (mean * mean);
  const stdDev = Math.sqrt(variance);

  // 2. Silverman's Rule for bandwidth if not provided
  // h = 1.06 * sigma * n^(-1/5)
  // We use effective sample size? Or just sum of weights if weights are counts?
  // Assuming weights are token counts, valid 'n' is debatable.
  // If we treat it as total tokens, n is huge and h is tiny -> spiky.
  // If we treat it as number of cases, h is larger -> smooth.
  // Let's use array length (number of data points) as 'n' for smoothing purposes,
  // because we are estimating the distribution of the *cases* weighted by importance,
  // not distinguishing every single token as an independent observation for smoothing.
  const n = data.length;
  const h = bandwidth || (1.06 * (stdDev || 10) * Math.pow(n, -0.2)); // Fallback stdDev if 0

  // 3. Gaussian Kernel
  const kernel = (u: number) => {
    return (1 / Math.sqrt(2 * Math.PI)) * Math.exp(-0.5 * u * u);
  };

  // 4. Compute Density
  const [min, max] = domain;
  const step = (max - min) / (ticks - 1);
  const density: [number, number][] = [];

  for (let i = 0; i < ticks; i++) {
    const x = min + i * step;
    let sum = 0;

    for (const d of data) {
      const u = (x - d.value) / h;
      sum += d.weight * kernel(u);
    }

    // Normalization: sum(weight) * h
    const y = sum / (totalWeight * h);
    density.push([x, y]);
  }

  return density;
}
