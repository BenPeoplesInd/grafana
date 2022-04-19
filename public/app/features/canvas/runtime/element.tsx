import React, { CSSProperties } from 'react';
import { OnDrag, OnResize } from 'react-moveable/declaration/types';

import {
  BackgroundImageSize,
  CanvasElementItem,
  CanvasElementOptions,
  canvasElementRegistry,
} from 'app/features/canvas';
import { DimensionContext } from 'app/features/dimensions';
import { notFoundItem } from 'app/features/canvas/elements/notFound';
import { GroupState } from './group';
import { LayerElement } from 'app/core/components/Layers/types';
import { Scene } from './scene';
import { HorizontalConstraint, Placement, VerticalConstraint } from '../types';

let counter = 0;

export class ElementState implements LayerElement {
  // UID necessary for moveable to work (for now)
  readonly UID = counter++;
  revId = 0;
  sizeStyle: CSSProperties = {};
  dataStyle: CSSProperties = {};

  // Filled in by ref
  div?: HTMLDivElement;

  // Calculated
  width = 100;
  height = 100;
  data?: any; // depends on the type

  constructor(public item: CanvasElementItem, public options: CanvasElementOptions, public parent?: GroupState) {
    const fallbackName = `Element ${Date.now()}`;
    if (!options) {
      this.options = { type: item.id, name: fallbackName };
    }

    options.constraint = options.constraint ?? {
      vertical: VerticalConstraint.Top,
      horizontal: HorizontalConstraint.Left,
    };
    options.placement = options.placement ?? { width: 100, height: 100, top: 0, left: 0 };

    const scene = this.getScene();
    if (!options.name) {
      const newName = scene?.getNextElementName();
      options.name = newName ?? fallbackName;
    }
    scene?.byName.set(options.name, this);
  }

  private getScene(): Scene | undefined {
    let trav = this.parent;
    while (trav) {
      if (trav.isRoot()) {
        return trav.scene;
        break;
      }
      trav = trav.parent;
    }

    return undefined;
  }

  getName() {
    return this.options.name;
  }

  validatePlacement() {
    let { constraint, placement } = this.options;
    placement = placement ?? {};
    const { vertical, horizontal } = constraint ?? {};
    const isConstrainedTop = vertical === VerticalConstraint.Top || vertical === VerticalConstraint.TopBottom;
    const isConstrainedBottom = vertical === VerticalConstraint.Bottom || vertical === VerticalConstraint.TopBottom;
    const isConstrainedLeft = horizontal === HorizontalConstraint.Left || horizontal === HorizontalConstraint.LeftRight;
    const isConstrainedRight =
      horizontal === HorizontalConstraint.Right || horizontal === HorizontalConstraint.LeftRight;

    const scene = this.getScene();
    const elementContainer = this.div && this.div.getBoundingClientRect();
    const rootContainer = scene?.div && this.getScene()?.div?.getBoundingClientRect();

    const relativeTop =
      elementContainer && rootContainer ? Math.abs(Math.round(elementContainer.top - rootContainer.top)) : 0;
    const relativeBottom =
      elementContainer && rootContainer ? Math.abs(Math.round(elementContainer.bottom - rootContainer.bottom)) : 0;
    const relativeLeft =
      elementContainer && rootContainer ? Math.abs(Math.round(elementContainer.left - rootContainer.left)) : 0;
    const relativeRight =
      elementContainer && rootContainer ? Math.abs(Math.round(elementContainer.right - rootContainer.right)) : 0;

    const w = placement.width ?? 100;
    const h = placement.height ?? 100;
    if (isConstrainedTop) {
      if (!placement.top) {
        placement.top = relativeTop;
      }
      if (isConstrainedBottom) {
        delete placement.height;
      } else {
        placement.height = h;
        delete placement.bottom;
      }
    } else if (isConstrainedBottom) {
      if (!placement.bottom) {
        placement.bottom = relativeBottom;
      }
      placement.height = h;
      delete placement.top;
    }
    if (isConstrainedLeft) {
      if (!placement.left) {
        placement.left = relativeLeft;
      }
      if (isConstrainedRight) {
        delete placement.width;
      } else {
        placement.width = w;
        delete placement.right;
      }
    } else if (isConstrainedRight) {
      if (!placement.right) {
        placement.right = relativeRight;
      }
      placement.width = w;
      delete placement.left;
    }
    this.width = w;
    this.height = h;
    this.options.placement = placement;
    this.revId++;
    scene && scene.revId++;
  }

  // The parent size, need to set our own size based on offsets
  updateSize(width: number, height: number) {
    this.width = width;
    this.height = height;
    this.validatePlacement();

    // Update the CSS position
    this.sizeStyle = {
      ...this.options.placement,
      position: 'absolute',
    };
  }

  updateData(ctx: DimensionContext) {
    if (this.item.prepareData) {
      this.data = this.item.prepareData(ctx, this.options.config);
      this.revId++; // rerender
    }

    const { background, border } = this.options;
    const css: CSSProperties = {};
    if (background) {
      if (background.color) {
        const color = ctx.getColor(background.color);
        css.backgroundColor = color.value();
      }
      if (background.image) {
        const image = ctx.getResource(background.image);
        if (image) {
          const v = image.value();
          if (v) {
            css.backgroundImage = `url("${v}")`;
            switch (background.size ?? BackgroundImageSize.Contain) {
              case BackgroundImageSize.Contain:
                css.backgroundSize = 'contain';
                css.backgroundRepeat = 'no-repeat';
                break;
              case BackgroundImageSize.Cover:
                css.backgroundSize = 'cover';
                css.backgroundRepeat = 'no-repeat';
                break;
              case BackgroundImageSize.Original:
                css.backgroundRepeat = 'no-repeat';
                break;
              case BackgroundImageSize.Tile:
                css.backgroundRepeat = 'repeat';
                break;
              case BackgroundImageSize.Fill:
                css.backgroundSize = '100% 100%';
                break;
            }
          }
        }
      }
    }

    if (border && border.color && border.width) {
      const color = ctx.getColor(border.color);
      css.borderWidth = border.width;
      css.borderStyle = 'solid';
      css.borderColor = color.value();

      // Move the image to inside the border
      if (css.backgroundImage) {
        css.backgroundOrigin = 'padding-box';
      }
    }

    this.dataStyle = css;
  }

  /** Recursively visit all nodes */
  visit(visitor: (v: ElementState) => void) {
    visitor(this);
  }

  onChange(options: CanvasElementOptions) {
    if (this.item.id !== options.type) {
      this.item = canvasElementRegistry.getIfExists(options.type) ?? notFoundItem;
    }

    // rename handling
    const oldName = this.options.name;
    const newName = options.name;

    this.revId++;
    this.options = { ...options };
    let trav = this.parent;
    while (trav) {
      if (trav.isRoot()) {
        trav.scene.save();
        break;
      }
      trav.revId++;
      trav = trav.parent;
    }

    const scene = this.getScene();
    if (oldName !== newName && scene) {
      scene.byName.delete(oldName);
      scene.byName.set(newName, this);
    }
  }

  getSaveModel() {
    return { ...this.options };
  }

  initElement = (target: HTMLDivElement) => {
    this.div = target;
  };

  applyDrag = (event: OnDrag) => {
    const { options } = this;
    const { placement, constraint } = options;
    const { vertical, horizontal } = constraint ?? {};

    const deltaX = event.delta[0];
    const deltaY = event.delta[1];

    const style = event.target.style;

    const isConstrainedTop = vertical === VerticalConstraint.Top || vertical === VerticalConstraint.TopBottom;
    const isConstrainedBottom = vertical === VerticalConstraint.Bottom || vertical === VerticalConstraint.TopBottom;
    const isConstrainedLeft = horizontal === HorizontalConstraint.Left || horizontal === HorizontalConstraint.LeftRight;
    const isConstrainedRight =
      horizontal === HorizontalConstraint.Right || horizontal === HorizontalConstraint.LeftRight;

    if (isConstrainedTop) {
      placement!.top! += deltaY;
      style.top = `${placement!.top}px`;
    }

    if (isConstrainedBottom) {
      placement!.bottom! -= deltaY;
      style.bottom = `${placement!.bottom}px`;
    }

    if (isConstrainedLeft) {
      placement!.left! += deltaX;
      style.left = `${placement!.left}px`;
    }

    if (isConstrainedRight) {
      placement!.right! -= deltaX;
      style.right = `${placement!.right}px`;
    }

    // TODO: Center + Scale
  };

  // kinda like:
  // https://github.com/grafana/grafana-edge-app/blob/main/src/panels/draw/WrapItem.tsx#L44
  applyResize = (event: OnResize) => {
    const { options } = this;
    const { placement, constraint: layout } = options;
    const { vertical, horizontal } = layout ?? {};

    const top = vertical === VerticalConstraint.Top || vertical === VerticalConstraint.TopBottom;
    const bottom = vertical === VerticalConstraint.Bottom || vertical === VerticalConstraint.TopBottom;
    const left = horizontal === HorizontalConstraint.Left || horizontal === HorizontalConstraint.LeftRight;
    const right = horizontal === HorizontalConstraint.Right || horizontal === HorizontalConstraint.LeftRight;

    const style = event.target.style;
    const deltaX = event.delta[0];
    const deltaY = event.delta[1];
    const dirLR = event.direction[0];
    const dirTB = event.direction[1];
    if (dirLR === 1) {
      // RIGHT
      if (right) {
        placement!.right! -= deltaX;
        style.right = `${placement!.right}px`;
        if (!left) {
          placement!.width = event.width;
          style.width = `${placement!.width}px`;
        }
      } else {
        placement!.width! = event.width;
        style.width = `${placement!.width}px`;
      }
    } else if (dirLR === -1) {
      // LEFT
      if (left) {
        placement!.left! -= deltaX;
        placement!.width! = event.width;
        style.left = `${placement!.left}px`;
        style.width = `${placement!.width}px`;
      } else {
        placement!.width! += deltaX;
        style.width = `${placement!.width}px`;
      }
    }

    if (dirTB === -1) {
      // TOP
      if (top) {
        placement!.top! -= deltaY;
        placement!.height = event.height;
        style.top = `${placement!.top}px`;
        style.height = `${placement!.height}px`;
      } else {
        placement!.height = event.height;
        style.height = `${placement!.height}px`;
      }
    } else if (dirTB === 1) {
      // BOTTOM
      if (bottom) {
        placement!.bottom! -= deltaY;
        placement!.height! = event.height;
        style.bottom = `${placement!.bottom}px`;
        style.height = `${placement!.height}px`;
      } else {
        placement!.height! = event.height;
        style.height = `${placement!.height}px`;
      }
    }

    // TODO: Center + Scale

    this.width = event.width;
    this.height = event.height;
  };

  render() {
    const { item } = this;
    return (
      <div key={`${this.UID}`} style={{ ...this.sizeStyle, ...this.dataStyle }} ref={this.initElement}>
        <item.display
          key={`${this.UID}/${this.revId}`}
          config={this.options.config}
          width={this.width}
          height={this.height}
          data={this.data}
        />
      </div>
    );
  }
}
