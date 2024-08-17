Elasticity is the relationship between price and volume. But as a business,
we're actually interested in the relationship between price and profit (which is
a function of volume, price, and cost).

```
profit = # widgets x widget price x widget margin

profit = # widgets x (widget price - widget cost)

old profit = Q x (P - C)

original margin = (P - C)/P = 1 - C/P

cost ratio = C/P

Q, P, C > 0

P > C

elasticity = dQ/dP

new profit = (1+dQ)Q x ((1-dP)P - C)

new profit = (1+dQ)(1-dP)QP - (1+dQ)QC

new profit = old profit + delta

delta = new - old

delta = (1+dQ)(1-dP)QP - (1+dQ)QC - QP + QC

delta = Q(((1+dQ)(1-dP) - 1)P - (dQ)C)

if delta > 0: ((1+dQ)(1-dP) - 1)P > (dQ)C

((1+dQ)(1-dP) - 1)/dQ > C/P

1 - ((1+dQ)(1-dP) - 1)/dQ < 1 - C/P

original margin > 1 - ((1+dQ)(1-dP) - 1)/dQ

original margin > 1 - (dP+dQ-dQdP)/dQ

original margin > - dP/dQ + dP

original margin > - dP(1/dQ + 1) # this comparator flips if dQ < 0 (why?)

0 < margin < 1

original margin > dP(1/dQ - 1)
```

```python
import numpy as np
import matplotlib.pyplot as plt

n = 1000
price_range = (-1, 1 + 1/n)
qty_range = (-1, 5 + 1/n)
price_values = np.linspace(*price_range, n)
qty_values = np.linspace(*qty_range, n)
margin_values = np.arange(0.1, 1.00, 0.1)

P, Q = np.meshgrid(price_values, qty_values)

RHS = -P * (1/Q + 1)

fig, ax = plt.subplots(figsize=(10, 6))

for i, margin in enumerate(margin_values):
    mask = np.where(Q >= 0, margin > RHS, margin < RHS)
    ax.contourf(P, Q, mask, levels=[0.5, 1], alpha=0.07)

contours = []
for margin in margin_values:
    mask = np.where(Q >= 0, margin > RHS, margin < RHS)
    contour = ax.contour(P, Q, mask, levels=[0], colors='black', linewidths=0.5)
    contours.append(contour)

for i, contour in enumerate(contours):
    suffix = ' margin' if i == 0 else ''
    paths = contour.collections[0].get_paths()
    label_positions = [path.vertices[int(0.1 * len(path.vertices))] for path in paths]
    ax.clabel(contour, inline=True, fontsize=8, fmt={0: f'{margin_values[i]:.0%}{suffix}'}, manual=label_positions, use_clabeltext=True)

ax.set_xlabel('Change in Price')
ax.set_ylabel('Change in Quantity')
ax.set_xticks(np.arange(*price_range, 0.1), [f'{x:.0%}' for x in np.arange(*price_range, 0.1)])
ax.set_yticks(np.arange(*qty_range, 1), [f'{y:.0%}' for y in np.arange(*qty_range, 1)])
ax.set_title('Profitable regions given change in price & qty for set margin')
ax.set_grid(True)

plt.show()
```
