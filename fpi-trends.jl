using DataFrames
using DataFramesMeta
using Gadfly
using Colors
using ColorTypes

const hoosierCrimson = RGB(([125, 17, 12] ./ 255)...)
const terrapinRed = RGB(([224, 58, 62] ./ 255)...)
const wolverineBlue = RGB(([0, 39, 76] ./ 255)...)
const spartanGreen = RGB(([24, 69, 59] ./ 255)...)
const buckeyeGrey = RGB(([102, 102, 102] ./ 255)...)
const lionBlue = RGB(([1, 45, 98] ./ 255)...)
const knightScarlet = RGB(([204, 0, 51] ./ 255)...)
const illiniOrange = RGB(([224, 78, 57] ./ 255)...)
const hawkeyeBlack = RGB(([0, 0, 0] ./ 255)...)
const gopherMaroon = RGB(([122, 0, 25] ./ 255)...)
const huskerScarlet = RGB(([208, 0, 0] ./ 255)...)
const wildcatPurple = RGB(([78, 42, 132] ./ 255)...)
const boilermakerGold = RGB(([206, 184, 136] ./ 255)...)
const badgerRed = RGB(([183, 1, 1] ./ 255)...)

const fpiFile = ARGS[1]
df = readtable(fpiFile, header = false)

# Rename fields to useful values
rename!(df, :x1, :team)
for i in 2:15
  rename!(df, symbol("x$i"), symbol("$(i-1)"))
end

# Stack on these to make the table long
df = stack(df)
rename!(df, [:value, :variable], [:fpi, :week])

# Fix the week to be an integer
function fixWeek(da::Vector{Symbol})
  result = DataArray(Int32, length(da))
  for i in 1:length(da)
    result[i] = parse(Int32, string(da[i]))
  end
  return result
end
df[:week] = fixWeek(df[:week])

# Stringify FPI for labeling
function stringify(da::DataArray{Float64, 1})
  result = DataArray(UTF8String, length(da))
  for i in 1:length(da)
    result[i] = string(da[i])
  end
  return result
end
df[:fpiString] = stringify(df[:fpi])

const b1gTeams = UTF8String["Illinois", "Indiana", "Iowa", "Maryland", "Michigan", "Michigan State", "Minnesota", "Nebraska", "Northwestern", "OSU", "Penn State", "Purdue", "Rutgers", "Wisconsin"]
const b1gColors = RGB[illiniOrange, hoosierCrimson, hawkeyeBlack, terrapinRed, wolverineBlue, spartanGreen, gopherMaroon, huskerScarlet, wildcatPurple, buckeyeGrey, lionBlue, boilermakerGold, knightScarlet, badgerRed]
const b1gdf = @where(df, findin(:team, b1gTeams))
const theme = Theme(background_color=colorant"white",
  key_position=:none,
  grid_line_width=.85pt,
  minor_label_font="'Cantarell','Calibri',sans-serif",
  major_label_font="'Cantarell','Calibri',sans-serif"
  )

# Label the deltas
const b1gDeltas = DataFrame([UTF8String, Int32, Float64], [:team, :week, :delta], 0)
for t in b1gTeams
  for w in 1:13
    delta = @ix(b1gdf, (:team .== t) & (:week .== w+1), :fpi)[1] - @ix(b1gdf, (:team .== t) & (:week .== w), :fpi)[1]
    append!(b1gDeltas, DataFrame(team=UTF8String(t), week=Int32(w), delta=Float64(delta)))
  end
end

# Spaghetti plot, B1G only
p1 = plot(layer(b1gdf,
  Geom.line, x=:week, y=:fpi, color=:team),
  layer(@where(b1gdf, findin(:week, [1, 14])),
  Geom.label(hide_overlaps=false, position=:dynamic), x=:week, y=:fpi, label=:team),
  layer(@where(b1gdf, findin(:week, [1, 14])),
  Geom.label(hide_overlaps=false, position=:dynamic), x=:week, y=:fpi, label=:fpiString),
  theme,
  Scale.color_discrete_manual(b1gColors...; levels=b1gTeams))
draw(SVG("fpi_spaghetti.svg", (10*golden)cm, 10cm), p1)

# Deltas plots
for t in b1gTeams
  p2 = plot(@where(b1gDeltas, :team .== t),
    Geom.line, Stat.step, x=:week, y=:delta, color=:team,
    Geom.hline, yintercept=[0],
    Guide.title("$t delta FPI"),
    theme,
    Scale.color_discrete_manual(b1gColors...; levels=b1gTeams))
  draw(SVG("$(t)_delta.svg", golden*10cm, 7cm), p2)
end

p3 = plot(b1gDeltas,
  Geom.boxplot, x=:team, y=:delta, color=:team,
  theme,
  Scale.color_discrete_manual(b1gColors...; levels=b1gTeams))
draw(SVG("all_deltas.svg", golden*10cm, 7cm), p3)

# B1G+overall absolute plot (for Ron)
const focuseddf = DataFrame(df)
@byrow! focuseddf begin
  if !in(:team, b1gTeams)
    :team = "FBS"
  end
end

p4 = plot(layer(df,
  Geom.violin, x=:week, y=:fpi,
  Geom.hline, yintercept=[0]),
  layer(b1gdf,
  Geom.violin, x=:week, y=:fpi),
  theme)
draw(SVG("fpi_distributions.svg", golden*10cm, 7cm), p4)

p5 = plot(layer(df,
  Geom.boxplot, x=:week, y=:fpi,
  Geom.hline, yintercept=[0]),
  layer(b1gdf,
  Geom.boxplot, x=:week, y=:fpi),
  theme)
draw(SVG("fpi_boxes.svg", golden*10cm, 7cm), p5)
