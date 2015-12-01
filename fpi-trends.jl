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

# Spaghetti plot, B1G only
const b1gTeams = UTF8String["Illinois", "Indiana", "Iowa", "Maryland", "Michigan", "Michigan State", "Minnesota", "Nebraska", "Northwestern", "OSU", "Penn State", "Purdue", "Rutgers", "Wisconsin"]
const b1gColors = RGB[illiniOrange, hoosierCrimson, hawkeyeBlack, terrapinRed, wolverineBlue, spartanGreen, gopherMaroon, huskerScarlet, wildcatPurple, buckeyeGrey, lionBlue, boilermakerGold, knightScarlet, badgerRed]
const b1gdf = @where(df, findin(:team, b1gTeams))
const theme = Theme(background_color=colorant"white", key_position=:none)

# Label the largest pre-post differences
const spans = Dict{Float64, AbstractString}()
for t in b1gTeams
  span = @ix(df, (:team .== t) & (:week .== 14), :fpi)[1] - @ix(df, (:team .== t) & (:week .== 1), :fpi)[1]
  spans[span] = t
end
const worstSpan = minimum(keys(spans))
const worstSpanTeam = spans[worstSpan]
const leftWorstSpan = @ix(df, (:team .== worstSpanTeam) & (:week .== 1), :fpi)[1]
const rightWorstSpan = @ix(df, (:team .== worstSpanTeam) & (:week .== 14), :fpi)[1]
const bestSpan = maximum(keys(spans))
const bestSpanTeam = spans[worstSpan]
const leftBestSpan = @ix(df, (:team .== bestSpanTeam) & (:week .== 1), :fpi)[1]
const rightBestSpan = @ix(df, (:team .== bestSpanTeam) & (:week .== 14), :fpi)[1]

p1 = plot(layer(b1gdf,
  Geom.line, x=:week, y=:fpi, color=:team),
  layer(@where(b1gdf, findin(:week, [1, 14])),
  Geom.label(hide_overlaps=false, position=:dynamic), x=:week, y=:fpi, label=:team),
  theme,
  Scale.color_discrete_manual(b1gColors...; levels=b1gTeams))
draw(PNG("fpi_spaghetti.png", 8inch, 6inch), p1)
